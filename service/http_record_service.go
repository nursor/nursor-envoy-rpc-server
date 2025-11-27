package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nursor-envoy-rpc/models/nursor"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HttpRecordService manages HTTP record pushing to external service.
type HttpRecordService struct {
	httpRecordUrl string
	initialized   bool
}

// singleton instance
var httpRecordInstance *HttpRecordService
var httpRecordOnce sync.Once

// GetHttpRecordInstance returns the singleton instance of HttpRecordService.
func GetHttpRecordInstance() *HttpRecordService {
	httpRecordOnce.Do(func() {
		httpRecordInstance = &HttpRecordService{}
		httpRecordInstance.initialize()
	})
	return httpRecordInstance
}

// initialize sets up the HttpRecordService with configuration.
func (hrs *HttpRecordService) initialize() {
	if hrs.initialized {
		return
	}
	hrs.httpRecordUrl = os.Getenv("ACCOUNT_MANAGER_URL")
	if hrs.httpRecordUrl == "" {
		hrs.httpRecordUrl = "http://172.16.238.2:31219/"
	}
	hrs.initialized = true
}

// InitializeForTest initializes the service for testing purposes with a custom URL
func (hrs *HttpRecordService) InitializeForTest(url string) {
	hrs.httpRecordUrl = url
	hrs.initialized = true
}

// HttpRecordPayload represents the payload format expected by the HTTP record API
type HttpRecordPayload struct {
	RequestHeaders  map[string]string `json:"request_headers"`
	RequestBody     string            `json:"request_body"` // Base64 encoded
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    string            `json:"response_body"` // Base64 encoded
	Url             string            `json:"url"`
	Method          string            `json:"method"`
	Host            string            `json:"host"`
	Datetime        int64             `json:"datetime"` // Unix timestamp
	HttpVersion     string            `json:"http_version"`
	AccountID       int               `json:"account_id"`
	UserID          int               `json:"user_id"`
	Status          int               `json:"status"`
}

// PushHttpRecord pushes an HTTP record to the external service.
func (hrs *HttpRecordService) PushHttpRecord(ctx context.Context, record *nursor.HttpRecord) error {
	if record == nil {
		return fmt.Errorf("http record is nil")
	}

	// Convert HttpRecord to API payload format
	payload := HttpRecordPayload{
		RequestHeaders:  record.RequestHeaders,
		ResponseHeaders: record.ResponseHeaders,
		Url:             record.Url,
		Method:          record.Method,
		Host:            record.Host,
		HttpVersion:     record.HttpVersion,
		AccountID:       record.AccountId,
		UserID:          record.UserId,
		Status:          record.Status,
	}

	// Encode request body to base64
	if len(record.RequestBody) > 0 {
		payload.RequestBody = base64.StdEncoding.EncodeToString(record.RequestBody)
	} else {
		payload.RequestBody = ""
	}

	// Encode response body to base64
	if len(record.ResponseBody) > 0 {
		payload.ResponseBody = base64.StdEncoding.EncodeToString(record.ResponseBody)
	} else {
		payload.ResponseBody = ""
	}

	// Convert CreateAt string to Unix timestamp
	if record.CreateAt != "" {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", record.CreateAt)
		if err != nil {
			// If parsing fails, use current time
			payload.Datetime = time.Now().Unix()
		} else {
			payload.Datetime = parsedTime.Unix()
		}
	} else {
		payload.Datetime = time.Now().Unix()
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Build URL
	url := hrs.httpRecordUrl
	if url == "" {
		return fmt.Errorf("http record URL is not configured")
	}
	if url[len(url)-1] != '/' {
		url += "/"
	}
	url += "http-record"

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Send request
	logrus.Debugf("Pushing HTTP record to %s", url)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle error responses
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http record service returned error (status %d): %s", resp.StatusCode, string(body))
	}

	// Check for successful response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	logrus.Debugf("Successfully pushed HTTP record for user %d, account %d", record.UserId, record.AccountId)
	return nil
}
