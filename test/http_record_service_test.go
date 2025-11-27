package test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nursor-envoy-rpc/models/nursor"
	"nursor-envoy-rpc/service"
	"os"
	"testing"
	"time"
)

// TestPushHttpRecord_Success tests successful HTTP record push
func TestPushHttpRecord_Success(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/http-record" {
			t.Errorf("Expected path /http-record, got %s", r.URL.Path)
		}

		// Verify request body
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify required fields
		if payload["url"] == nil {
			t.Error("Expected url field in payload")
		}
		if payload["method"] == nil {
			t.Error("Expected method field in payload")
		}
		if payload["account_id"] == nil {
			t.Error("Expected account_id field in payload")
		}
		if payload["user_id"] == nil {
			t.Error("Expected user_id field in payload")
		}
		if payload["datetime"] == nil {
			t.Error("Expected datetime field in payload")
		}

		// Verify base64 encoded bodies
		if requestBody, ok := payload["request_body"].(string); ok && requestBody != "" {
			_, err := base64.StdEncoding.DecodeString(requestBody)
			if err != nil {
				t.Errorf("request_body should be base64 encoded: %v", err)
			}
		}

		if responseBody, ok := payload["response_body"].(string); ok && responseBody != "" {
			_, err := base64.StdEncoding.DecodeString(responseBody)
			if err != nil {
				t.Errorf("response_body should be base64 encoded: %v", err)
			}
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set environment variable for test
	// os.Setenv("HTTP_RECORD_URL", server.URL+"/")
	// defer os.Unsetenv("HTTP_RECORD_URL")

	// Create service instance directly for testing
	// hrs := &service.HttpRecordService{}

	// hrs.InitializeForTest(server.URL + "/")

	hrs := service.GetHttpRecordInstance()

	// Create test HTTP record
	record := nursor.NewRequestRecord()
	record.Url = "http://example.com/test"
	record.Method = "POST"
	record.Host = "example.com"
	record.UserId = 80
	record.AccountId = 775
	record.Status = 200
	record.CreateAt = time.Now().Format("2006-01-02 15:04:05")
	record.AddRequestHeader("Content-Type", "application/json")
	record.AddRequestBody([]byte("test request body"))
	record.AddResponseHeader("Content-Type", "application/json")
	record.AddResponseBody([]byte("test response body"))

	// Test
	ctx := context.Background()
	err := hrs.PushHttpRecord(ctx, record)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestPushHttpRecord_ErrorResponse tests error response
func TestPushHttpRecord_ErrorResponse(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest) // 400
		w.Write([]byte(`{"error": "Invalid request"}`))
	}))
	defer server.Close()

	// Set environment variable for test
	os.Setenv("HTTP_RECORD_URL", server.URL+"/")
	defer os.Unsetenv("HTTP_RECORD_URL")

	// Create service instance directly for testing
	hrs := &service.HttpRecordService{}
	hrs.InitializeForTest(server.URL + "/")

	// Create test HTTP record
	record := nursor.NewRequestRecord()
	record.Url = "http://example.com/test"
	record.Method = "POST"
	record.Host = "example.com"
	record.UserId = 80
	record.AccountId = 775
	record.Status = 200

	// Test
	ctx := context.Background()
	err := hrs.PushHttpRecord(ctx, record)

	// Assertions
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// TestPushHttpRecord_NilRecord tests error when record is nil
func TestPushHttpRecord_NilRecord(t *testing.T) {
	// Create service instance directly for testing
	hrs := &service.HttpRecordService{}
	hrs.InitializeForTest("http://127.0.0.1:3001/")

	// Test
	ctx := context.Background()
	err := hrs.PushHttpRecord(ctx, nil)

	// Assertions
	if err == nil {
		t.Fatal("Expected error when record is nil, got nil")
	}
}

// TestPushHttpRecord_EmptyBodies tests with empty request and response bodies
func TestPushHttpRecord_EmptyBodies(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify empty bodies are sent as empty strings
		if requestBody, ok := payload["request_body"].(string); !ok || requestBody != "" {
			t.Errorf("Expected empty request_body, got: %v", payload["request_body"])
		}
		if responseBody, ok := payload["response_body"].(string); !ok || responseBody != "" {
			t.Errorf("Expected empty response_body, got: %v", payload["response_body"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set environment variable for test
	os.Setenv("HTTP_RECORD_URL", server.URL+"/")
	defer os.Unsetenv("HTTP_RECORD_URL")

	// Create service instance directly for testing
	hrs := &service.HttpRecordService{}
	hrs.InitializeForTest(server.URL + "/")

	// Create test HTTP record with empty bodies
	record := nursor.NewRequestRecord()
	record.Url = "http://example.com/test"
	record.Method = "GET"
	record.Host = "example.com"
	record.UserId = 80
	record.AccountId = 775
	record.Status = 200
	record.CreateAt = time.Now().Format("2006-01-02 15:04:05")

	// Test
	ctx := context.Background()
	err := hrs.PushHttpRecord(ctx, record)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestPushHttpRecord_DatetimeConversion tests datetime conversion from CreateAt string
func TestPushHttpRecord_DatetimeConversion(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify datetime is a valid Unix timestamp
		if datetime, ok := payload["datetime"].(float64); ok {
			if datetime <= 0 {
				t.Errorf("Expected positive datetime, got: %v", datetime)
			}
			// Verify it's a reasonable timestamp (not too far in the future)
			now := time.Now().Unix()
			if datetime > float64(now+86400) { // More than 1 day in the future
				t.Errorf("Datetime seems too far in the future: %v", datetime)
			}
		} else {
			t.Error("Expected datetime field to be a number")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set environment variable for test
	os.Setenv("HTTP_RECORD_URL", server.URL+"/")
	defer os.Unsetenv("HTTP_RECORD_URL")

	// Create service instance directly for testing
	hrs := &service.HttpRecordService{}
	hrs.InitializeForTest(server.URL + "/")

	// Create test HTTP record with specific CreateAt time
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	record := nursor.NewRequestRecord()
	record.Url = "http://example.com/test"
	record.Method = "GET"
	record.Host = "example.com"
	record.UserId = 80
	record.AccountId = 775
	record.Status = 200
	record.CreateAt = testTime.Format("2006-01-02 15:04:05")

	// Test
	ctx := context.Background()
	err := hrs.PushHttpRecord(ctx, record)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestPushHttpRecord_NoURL tests error when URL is not configured
func TestPushHttpRecord_NoURL(t *testing.T) {
	// Unset environment variable
	os.Unsetenv("HTTP_RECORD_URL")

	// Create a new service instance with empty URL
	hrs := &service.HttpRecordService{}
	hrs.InitializeForTest("")

	// Create test HTTP record
	record := nursor.NewRequestRecord()
	record.Url = "http://example.com/test"
	record.Method = "POST"
	record.Host = "example.com"
	record.UserId = 80
	record.AccountId = 775
	record.Status = 200

	// Test
	ctx := context.Background()
	err := hrs.PushHttpRecord(ctx, record)

	// Assertions
	if err == nil {
		t.Fatal("Expected error when URL is not configured, got nil")
	}
}
