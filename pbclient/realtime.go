package pbclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type RealtimeService struct {
	pb            *Pocketbase
	clientID      string
	subscriptions map[string][]func(data interface{})
	mu            sync.RWMutex
	eventSource   *eventSource
	clientIDChan  chan string
}

type eventSource struct {
	url        string
	connection *http.Response
	reader     *bufio.Reader
}

func (pb *Pocketbase) NewRealtimeService() *RealtimeService {
	return &RealtimeService{
		pb:            pb,
		subscriptions: make(map[string][]func(data interface{})),
		clientIDChan:  make(chan string, 1),
	}
}

func (rs *RealtimeService) connect() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.eventSource != nil {
		return nil // Already connected
	}

	u, err := url.Parse(rs.pb.BaseEndpoint)
	if err != nil {
		return fmt.Errorf("invalid base endpoint: %w", err)
	}

	u.Path = "/api/realtime"

	// log.Printf("Attempting to connect to SSE URL: '%v'", u.String())

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return fmt.Errorf("err creating request : '%w'", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Authorization", rs.pb.AuthToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("err performing request : '%w'", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("unexpected status code: '%v'", resp.StatusCode)
	}

	rs.eventSource = &eventSource{
		url:        u.String(),
		connection: resp,
		reader:     bufio.NewReader(resp.Body),
	}

	go rs.readEvents()

	return nil
}

func (rs *RealtimeService) readEvents() {
	defer rs.eventSource.connection.Body.Close()

	for {
		line, err := rs.eventSource.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				log.Println("End of SSE stream")
				return
			}
			log.Printf("err reading SSE stream : '%v'\n", err)
			return
		}

		line = strings.TrimSpace(line)
		log.Printf("Received SSE line : '%v'", line)

		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		var event struct {
			ClientID string `json:"clientId"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			log.Printf("err unmarshaling event data : '%v'", err)
			continue
		}
		if event.ClientID != "" {
			rs.mu.Lock()
			rs.clientID = event.ClientID
			rs.mu.Unlock()
			log.Printf("received clientID: %v", rs.clientID)
			select {
			case rs.clientIDChan <- event.ClientID:
			default:
				// Channel is full, which means we've already sent the client ID
			}
		}
	}
}

func (rs *RealtimeService) Subscribe(topic string, callback func(data interface{})) error {
	if err := rs.connect(); err != nil {
		return err
	}

	rs.mu.Lock()
	rs.subscriptions[topic] = append(rs.subscriptions[topic], callback)
	rs.mu.Unlock()

	return rs.submitSubscriptions()
}

func (rs *RealtimeService) submitSubscriptions() error {
	var clientID string
	select {
	case clientID = <-rs.clientIDChan:
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timeout waiting for client ID")
	}

	rs.mu.RLock()
	topics := make([]string, 0, len(rs.subscriptions))
	for topic := range rs.subscriptions {
		topics = append(topics, topic)
	}
	rs.mu.RUnlock()

	// log.Printf("Submitting subscriptions with ClientID: '%v'\n", clientID)

	payload := struct {
		ClientID      string   `json:"clientId"`
		Subscriptions []string `json:"subscriptions"`
	}{
		ClientID:      clientID,
		Subscriptions: topics,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling subscription payload: %w", err)
	}

	// log.Printf("subscription payload: '%v'", string(jsonPayload))

	req, err := http.NewRequest("POST", rs.pb.BaseEndpoint+"/api/realtime", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("err creating request : '%w'", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", rs.pb.AuthToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("err performing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("err reading body of subscribe request : '%v'\n", err)
	}
	// log.Printf("subscription response: status : '%v', body : '%v'\n", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code : '%v', body : '%v'", resp.StatusCode, string(body))
	}

	// log.Printf("subscriptions submitted successfully\n")
	return nil
}
