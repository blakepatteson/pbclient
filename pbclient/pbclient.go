package pbclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Pocketbase struct {
	BaseEndpoint string
	Username     string
	AuthToken    string
}

type Params struct {
	Page   int
	Filter string
	Expand string
}

const (
	MAX_PER_PAGE        = 256
	ADMIN_AUTH_ENDPOINT = "/api/collections/_superusers/auth-with-password"
	AUTH_ENDPOINT       = "/api/collections/users/auth-with-password"
)

func NewPocketbase(baseUrl, un, pw string, isAdmin bool) (*Pocketbase, error) {
	var authToken string
	var err error
	if isAdmin {
		authToken, err = authenticate(ADMIN_AUTH_ENDPOINT, baseUrl, un, pw)
		if err != nil {
			return nil, err
		}
	} else {
		authToken, err = authenticate(AUTH_ENDPOINT, baseUrl, un, pw)
		if err != nil {
			return nil, err
		}
	}
	return &Pocketbase{
		BaseEndpoint: baseUrl,
		Username:     un,
		AuthToken:    authToken,
	}, nil
}

func NewPocketbaseFromToken(baseUrl, token string) *Pocketbase {
	if !strings.HasPrefix(token, "Bearer ") {
		token = fmt.Sprintf("Bearer %v", token)
	}
	return &Pocketbase{BaseEndpoint: baseUrl, AuthToken: token}
}

func authenticate(authEndpoint, baseEndpoint, id, pw string) (string, error) {
	authJson := fmt.Appendf(nil, `{"identity":"%v","password":"%v"}`, id, pw)
	resp, err := http.Post(
		fmt.Sprintf("%v%v", baseEndpoint, authEndpoint),
		"application/json",
		bytes.NewBuffer(authJson),
	)
	if err != nil {
		return "", fmt.Errorf("err authenticating to db : '%v'", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing auth resp body : '%v'\n", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("err reading response body : '%v'", err)
	}

	var respJson map[string]any
	if err := json.Unmarshal(body, &respJson); err != nil {
		return "", fmt.Errorf("err parsing response JSON : '%v'", err)
	}

	token, ok := respJson["token"]
	if !ok {
		return "", fmt.Errorf("token not found in response")
	}
	return fmt.Sprintf(`Bearer %v`, token), nil
}

func (pb *Pocketbase) getLogs(page int) ([]map[string]any, int, error) {
	allRecords, totalItems, err := pb.getData("/api/logs/requests/?page=%v",
		Params{Page: page, Filter: ""})
	if err != nil {
		return nil, -1, fmt.Errorf("err getting logs : '%v'", err)
	}
	return allRecords, totalItems, nil
}

func (pb *Pocketbase) CreateRecord(collectionName, update string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/collections/%v/records",
		pb.BaseEndpoint, collectionName)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(update))
	if err != nil {
		return "", fmt.Errorf("err creating request : '%v'", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", pb.AuthToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("err creating pb db record : '%v'", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing create resp body : '%v'\n", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("err reading response body : '%v'", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("err parsing resp json : '%v'", err)
	}
	if id, ok := result["id"]; ok {
		return id.(string), nil
	}

	return "", fmt.Errorf("err parsing id from pb db record")
}

func (pb *Pocketbase) GetAllLogs() ([]map[string]any, error) {
	results, totRecs, err := pb.getLogs(1)
	if err != nil {
		return nil, err
	}

	if len(results) < MAX_PER_PAGE || totRecs == MAX_PER_PAGE {
		return results, nil
	}
	allResults := results
	pg := 2
	for len(allResults) < totRecs {
		results, totRecs, err = pb.getLogs(pg)
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, results...)
		pg += 1
	}
	return allResults, nil
}

func GetAllTypedRecords[T any](
	pb *Pocketbase, collectionName, filter, expand string,
) ([]T, error) {
	params := Params{Page: 1, Expand: expand, Filter: filter}
	results, totRecs, err := getTypedRecords[T](pb, collectionName, params)
	if err != nil {
		return nil, err
	}

	if len(results) < MAX_PER_PAGE || totRecs == MAX_PER_PAGE || len(results) == 0 {
		return results, nil
	}
	allResults := results
	whichPage := 2
	for len(allResults) < totRecs {
		results, totRecs, err = getTypedRecords[T](pb, collectionName, Params{
			Page: whichPage, Expand: expand, Filter: filter,
		})
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, results...)
		whichPage += 1
	}
	return allResults, nil
}

func getTypedRecords[T any](
	pb *Pocketbase, collectionName string, params Params,
) ([]T, int, error) {
	getEndpoint := fmt.Sprintf("%s/api/collections/%s/records?page=%d&perPage=%d",
		pb.BaseEndpoint, collectionName, params.Page, MAX_PER_PAGE)
	if params.Filter != "" {
		getEndpoint += "&filter=" + url.QueryEscape(params.Filter)
	}
	if params.Expand != "" {
		getEndpoint += "&expand=" + url.QueryEscape(params.Expand)
	}

	req, err := http.NewRequest("GET", getEndpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("err creating request : '%v'", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", pb.AuthToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("err getting data from pb db : '%v'", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing getTypedRecs resp body : '%v'\n", err)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("received non-200 response status : %v",
			response.StatusCode)
	}

	var respMap struct {
		Items      []T `json:"items"`
		TotalItems int `json:"totalItems"`
	}

	if err := json.NewDecoder(response.Body).Decode(&respMap); err != nil {
		return nil, 0, fmt.Errorf("err parsing response JSON : %v", err)
	}

	return respMap.Items, respMap.TotalItems, nil
}
func (pb *Pocketbase) getData(
	getDataEndpoint string, params Params,
) ([]map[string]any, int, error) {
	// Build the endpoint URL with query parameters
	getEndpoint := fmt.Sprintf("%s%s?page=%d&perPage=%v",
		pb.BaseEndpoint, getDataEndpoint, params.Page, MAX_PER_PAGE)
	if params.Filter != "" {
		getEndpoint += "&filter=" + url.QueryEscape(params.Filter)
	}
	if params.Expand != "" {
		getEndpoint += "&expand=" + url.QueryEscape(params.Expand)
	}

	req, err := http.NewRequest("GET", getEndpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("err creating request : '%v'", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", pb.AuthToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("err getting data from pb db : '%v'", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing getData resp body : '%v'\n", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("err reading response body : '%v'", err)
	}

	var respMap map[string]any
	if err := json.Unmarshal(body, &respMap); err != nil {
		return nil, 0, fmt.Errorf("err parsing resp json : '%v'", err)
	}

	allRecords := []map[string]any{}
	if items, ok := respMap["items"].([]any); ok {
		for _, el := range items {
			if record, ok := el.(map[string]any); ok {
				allRecords = append(allRecords, record)
			}
		}
	}

	totalItems := 0
	if total, ok := respMap["totalItems"].(float64); ok {
		totalItems = int(total)
	}

	return allRecords, totalItems, nil
}

func (pb *Pocketbase) getRecords(
	collectionName string, params Params,
) ([]map[string]any, int, error) {
	return pb.getData(fmt.Sprintf("/api/collections/%v/records", collectionName), params)
}

func (pb *Pocketbase) GetRecordById(collectionName, id string) (map[string]any, error) {
	endpoint := fmt.Sprintf("%v/api/collections/%v/records/%v",
		pb.BaseEndpoint, collectionName, id)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("err creating request : '%v'", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", pb.AuthToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("err getting filtered db records : '%v'", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing getRecords resp body : '%v'\n", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("err reading response body : '%v'", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("err parsing response JSON : '%v'", err)
	}

	return result, nil
}

func (pb *Pocketbase) GetFilteredRecords(collectionName, filter string) (
	[]map[string]any, error,
) {
	endpoint := fmt.Sprintf("%v/api/collections/%v/records?page=1&filter=%v",
		pb.BaseEndpoint, collectionName, url.QueryEscape(filter))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("err creating request : '%v'", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", pb.AuthToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		fmt.Println("err getting filtered db records : ", err)
		return nil, fmt.Errorf("err getting filtered db records : '%v'", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing getFilteredRecs resp body : '%v'\n", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("err reading response body : '%v'", err)
	}

	var respMap map[string]any
	if err := json.Unmarshal(body, &respMap); err != nil {
		return nil, fmt.Errorf("err parsing resp json : '%v'", err)
	}

	filteredRecords := []map[string]any{}
	if respMap["items"] != nil {
		if items, ok := respMap["items"].([]any); ok {
			for _, el := range items {
				if record, ok := el.(map[string]any); ok {
					filteredRecords = append(filteredRecords, record)
				}
			}
		}
	} else {
		return nil, fmt.Errorf("err getting filtered records from pb db")
	}
	return filteredRecords, nil
}

func (pb *Pocketbase) GetAllRecords(collectionName, filter, expand string) (
	[]map[string]any, error,
) {
	params := Params{Page: 1, Expand: expand, Filter: filter}
	results, totRecs, err := pb.getRecords(collectionName, params)
	if err != nil {
		return nil, err
	}

	if len(results) < MAX_PER_PAGE || totRecs == MAX_PER_PAGE || len(results) == 0 {
		return results, nil
	}
	allResults := results
	whichPage := 2
	for len(allResults) < totRecs {
		results, totRecs, err = pb.getRecords(collectionName, Params{
			Page: whichPage, Expand: expand, Filter: filter,
		})
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, results...)
		whichPage += 1
	}
	return allResults, nil
}

func (pb *Pocketbase) UpdateRecord(collectionName, update, id string) (string, error) {
	endpoint := fmt.Sprintf("%v/api/collections/%v/records/%v",
		pb.BaseEndpoint, collectionName, id)

	req, err := http.NewRequest("PATCH", endpoint, bytes.NewBufferString(update))
	if err != nil {
		return "", fmt.Errorf("err creating request : '%v'", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", pb.AuthToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("err updating pb db record : '%v'", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing updateRec resp body : '%v'\n", err)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("err reading response body : '%v'", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("err parsing resp json : '%v'", err)
	}
	if id, ok := result["id"]; ok {
		return id.(string), nil
	}
	return "", fmt.Errorf("err parsing id from update pb db record")
}

func ParseTimePB(input string) (*time.Time, error) {
	time, err := time.Parse("2006-01-02 15:04:05.999Z", input)
	if err != nil {
		return nil, fmt.Errorf("err parsing time : '%v'", err)
	}
	return &time, nil
}

func (pb *Pocketbase) DeleteRecord(collectionName, recordId string) (int, error) {
	deleteEndpoint := fmt.Sprintf("%v/api/collections/%v/records/%v",
		pb.BaseEndpoint, collectionName, recordId)

	req, err := http.NewRequest("DELETE", deleteEndpoint, nil)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("err creating request : '%v'", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", pb.AuthToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("err deleting PB DB rec : '%v'", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing deleteRec resp body : '%v'\n", err)
		}
	}()

	return response.StatusCode, nil
}

func AuthRefresh(authToken, baseEndpoint string) (*Pocketbase, error) {
	endpt := fmt.Sprintf("%v/api/collections/users/auth-refresh", baseEndpoint)

	req, err := http.NewRequest("POST", endpt, nil)
	if err != nil {
		return nil, fmt.Errorf("err creating request : '%v'", err)
	}
	req.Header.Set("Authorization", authToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("err refreshing auth : '%v'", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("pbclient.[WARN] : err closing authRefresh resp body : '%v'\n", err)
		}
	}()

	return &Pocketbase{BaseEndpoint: baseEndpoint, AuthToken: authToken}, nil
}
