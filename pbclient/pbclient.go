package pbclient

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/blakepatteson/gorequests/requests"
)

type Pocketbase struct {
	BaseEndpoint string
	Username     string
	password     string
	AuthToken    string
}

type Params struct {
	Page   int
	Filter string
	Expand string
}

const (
	MAX_PER_PAGE        = 256
	ADMIN_AUTH_ENDPOINT = "/api/admins/auth-with-password"
	AUTH_ENDPOINT       = "/api/collections/users/auth-with-password"
)

func NewPocketbase(baseUrl, username, password string, isAdmin bool) (*Pocketbase, error) {
	var authToken string
	var err error
	if isAdmin {
		authToken, err = authenticate(ADMIN_AUTH_ENDPOINT, baseUrl, username, password)
		if err != nil {
			return nil, err
		}
	} else {
		authToken, err = authenticate(AUTH_ENDPOINT, baseUrl, username, password)
		if err != nil {
			return nil, err
		}
	}
	return &Pocketbase{
		BaseEndpoint: baseUrl,
		Username:     username,
		password:     password,
		AuthToken:    authToken,
	}, nil
}

func authenticate(authEndpoint, baseEndpoint, identity, password string) (string, error) {
	authJson := []byte(fmt.Sprintf(`{"identity":"%v","password":"%v"}`, identity, password))
	response, err := requests.HttpRequest{
		Endpoint:    fmt.Sprintf("%v%v", baseEndpoint, authEndpoint),
		VerbHTTP:    "POST",
		ContentType: "application/json",
		JSON:        authJson,
	}.Do()

	if err != nil {
		return "", fmt.Errorf("err authenticating to db : %w", err)
	}

	respJson, err := requests.ParseJson(response)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`Bearer %v`, respJson["token"]), nil
}

func (pb *Pocketbase) getLogs(page int) ([]map[string]any, int, error) {
	allRecords, totalItems, err := pb.getData("/api/logs/requests/?page=%v", Params{Page: page, Filter: ""})
	if err != nil {
		return nil, -1, fmt.Errorf("err getting logs : %w", err)
	}
	return allRecords, totalItems, nil
}

func (pb *Pocketbase) CreateRecord(collectionName, update string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/collections/%v/records", pb.BaseEndpoint, collectionName)
	response, err := requests.HttpRequest{
		Endpoint:    endpoint,
		ContentType: "application/json",
		VerbHTTP:    "POST",
		Auth:        pb.AuthToken,
		JSON:        []byte(update),
	}.Do()

	if err != nil {
		return "", fmt.Errorf("err creating pb db record : %w", err)
	}

	result, err := requests.ParseJson(response)
	if err != nil {
		return "", fmt.Errorf("err parsing resp json : '%v'", err)
	}
	if _, ok := result["id"]; ok {
		return result["id"].(string), nil
	}

	return "", fmt.Errorf("err parsing id from pb db record : %w", err)
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

func (pb *Pocketbase) getData(getDataEndpoint string, params Params) ([]map[string]any, int, error) {
	// Build the endpoint URL with query parameters
	getEndpoint := fmt.Sprintf("%s%s?page=%d&perPage=%v",
		pb.BaseEndpoint, getDataEndpoint, params.Page, MAX_PER_PAGE)
	if params.Filter != "" {
		getEndpoint += "&filter=" + url.QueryEscape(params.Filter)
	}
	if params.Expand != "" {
		getEndpoint += "&expand=" + url.QueryEscape(params.Expand)
	}
	response, err := requests.HttpRequest{
		Endpoint:    getEndpoint,
		ContentType: "application/json",
		VerbHTTP:    "GET",
		Auth:        pb.AuthToken,
	}.Do()
	if err != nil {
		return nil, 0, fmt.Errorf("err getting data from pb db : %w", err)
	}
	respMap, err := requests.ParseJson(response)
	if err != nil {
		return nil, 0, fmt.Errorf("err parsing resp json : '%v'", err)
	}
	allRecords := []map[string]any{}
	for _, el := range respMap["items"].([]any) {
		allRecords = append(allRecords, el.(map[string]any))
	}
	return allRecords, int(respMap["totalItems"].(float64)), nil
}

func (pb *Pocketbase) getRecords(collectionName string, params Params) ([]map[string]any, int, error) {
	return pb.getData(fmt.Sprintf("/api/collections/%v/records", collectionName), params)
}

func (pb *Pocketbase) GetRecordById(collectionName, id string) (map[string]any, error) {
	response, err := requests.HttpRequest{
		Endpoint: fmt.Sprintf("%v/api/collections/%v/records/%v",
			pb.BaseEndpoint, collectionName, id),
		ContentType: "application/json",
		VerbHTTP:    "GET",
		Auth:        pb.AuthToken,
	}.Do()
	if err != nil {
		fmt.Println("err getting filtered db records : ", err, response)
	}
	return requests.ParseJson(response)
}

func (pb *Pocketbase) GetFilteredRecords(collectionName, filter string) ([]map[string]any, error) {
	response, err := requests.HttpRequest{
		Endpoint: fmt.Sprintf("%v/api/collections/%v/records?page=1&filter=%v",
			pb.BaseEndpoint, collectionName, url.QueryEscape(filter)),
		ContentType: "application/json",
		VerbHTTP:    "GET",
		Auth:        pb.AuthToken,
	}.Do()
	if err != nil {
		fmt.Println("err getting filtered db records : ", err)
	}
	respMap, err := requests.ParseJson(response)
	if err != nil {
		return nil, fmt.Errorf("err parsing resp json : '%v'", err)
	}
	filteredRecords := []map[string]any{}
	if respMap["items"] != nil {
		for _, el := range respMap["items"].([]any) {
			filteredRecords = append(filteredRecords, el.(map[string]any))
		}
	} else {
		return nil, fmt.Errorf("err getting filtered records from pb db : %w", err)
	}
	return filteredRecords, nil
}

func (pb *Pocketbase) GetAllRecords(collectionName, filter string, expand string) ([]map[string]any, error) {
	params := Params{
		Page: 1, Expand: expand, Filter: filter,
	}
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
	endpoint := fmt.Sprintf("%v/api/collections/%v/records/%v", pb.BaseEndpoint, collectionName, id)
	response, err := requests.HttpRequest{
		Endpoint:    endpoint,
		ContentType: "application/json",
		VerbHTTP:    "PATCH",
		Auth:        pb.AuthToken,
		JSON:        []byte(update),
	}.Do()
	if err != nil {
		return "", fmt.Errorf("err updating pb db record : %w", err)
	}
	result, err := requests.ParseJson(response)
	if err != nil {
		return "", fmt.Errorf("err parsing resp json : '%v'", err)
	}
	if _, ok := result["id"]; ok {
		return result["id"].(string), nil
	}
	return "", fmt.Errorf("err parsing id from update pb db record : %w", err)
}

func ParseTimePB(input string) (*time.Time, error) {
	time, err := time.Parse("2006-01-02 15:04:05.999Z", input)
	if err != nil {
		return nil, fmt.Errorf("err parsing time : %w", err)
	}
	return &time, nil
}

func (pb *Pocketbase) DeleteRecord(collectionName, recordId string) (int, error) {
	deleteEndpoint := fmt.Sprintf("%v/api/collections/%v/records/%v", pb.BaseEndpoint, collectionName, recordId)
	response, err := requests.HttpRequest{
		Endpoint:    deleteEndpoint,
		ContentType: "application/json",
		VerbHTTP:    "DELETE",
		Auth:        pb.AuthToken,
	}.Do()
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("err deleting PB DB records. Details : '%v'", err)
	}
	return response.StatusCode, nil
}

func AuthRefresh(authToken, baseEndpoint string) (int, error) {
	endpt := fmt.Sprintf("%v/api/collections/users/auth-refresh", baseEndpoint)
	response, err := requests.HttpRequest{
		Endpoint: endpt,
		Auth:     authToken,
	}.Do()
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("err refreshing auth : '%v'", err)
	}

	return response.StatusCode, nil
}
