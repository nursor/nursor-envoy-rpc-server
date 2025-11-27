package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nursor-envoy-rpc/service"
	"os"
	"testing"
)

// TestGetAccountByUserId_Success tests successful account acquisition
func TestGetAccountByUserId_Success(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/acquire" {
			t.Errorf("Expected path /acquire, got %s", r.URL.Path)
		}

		// Verify request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Return successful response
		response := map[string]interface{}{
			"account": map[string]interface{}{
				"id":                 775,
				"email":              "test@example.com",
				"name":               "test",
				"password":           "password123",
				"cursor_id":          "1774",
				"first_name":         "Test",
				"last_name":          "User",
				"access_token":       "test_token",
				"sub_id":             "",
				"refresh_token":      "refresh_token",
				"membership_type":    "free_trial",
				"cache_email":        false,
				"unique_cpp_user_id": "123",
				"client_key":         "client_key_123",
				"dispatch_order":     0,
				"description":        "",
				"status":             "active",
				"expires_at":         nil,
				"created_at":         1746591538916,
				"updated_at":         1764125764111,
				"usage":              0,
				"detail_usage":       0,
				"usage_limit":        0,
			},
			"reused": false,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create service instance directly for testing
	ds := &service.DispatchService{}
	ds.InitializeForTest(server.URL + "/")

	// Test
	ctx := context.Background()
	account, err := ds.GetAccountByUserId(ctx, 80)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if account == nil {
		t.Fatal("Expected account to be not nil")
	}
	if account.ID != 775 {
		t.Errorf("Expected account ID 775, got %d", account.ID)
	}
	if account.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", account.Email)
	}
	if account.CursorID != "1774" {
		t.Errorf("Expected cursor_id 1774, got %s", account.CursorID)
	}
}

// TestGetAccountByUserId_ErrorResponse tests error response (402)
func TestGetAccountByUserId_ErrorResponse(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   "用户套餐已过期或使用次数已超限",
			"message": "您的套餐已过期或使用次数已达到上限，请续费或升级套餐",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired) // 402
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create service instance directly for testing
	ds := &service.DispatchService{}
	ds.InitializeForTest(server.URL + "/")

	// Test
	ctx := context.Background()
	account, err := ds.GetAccountByUserId(ctx, 80)

	// Assertions
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if account != nil {
		t.Error("Expected account to be nil on error")
	}
	if err.Error() == "" {
		t.Error("Expected error message, got empty string")
	}
}

// TestIncrTokenUsage_Success tests successful usage increment
func TestIncrTokenUsage_Success(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/usage/inc" {
			t.Errorf("Expected path /usage/inc, got %s", r.URL.Path)
		}

		// Verify request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		accountId, ok := reqBody["accountId"].(float64)
		if !ok || int(accountId) != 775 {
			t.Errorf("Expected accountId 775, got %v", reqBody["accountId"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create service instance directly for testing
	ds := &service.DispatchService{}
	ds.InitializeForTest(server.URL + "/")

	// Test
	ctx := context.Background()
	err := ds.IncrTokenUsage(ctx, 775)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestIncrTokenUsage_ErrorResponse tests error response
func TestIncrTokenUsage_ErrorResponse(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   "Account not found",
			"message": "The specified account does not exist",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound) // 404
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create service instance directly for testing
	ds := &service.DispatchService{}
	ds.InitializeForTest(server.URL + "/")

	// Test
	ctx := context.Background()
	err := ds.IncrTokenUsage(ctx, 999)

	// Assertions
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// TestHandleTokenExpired_Success tests successful account disable
func TestHandleTokenExpired_Success(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/account/775/disable" {
			t.Errorf("Expected path /account/775/disable, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create service instance directly for testing
	ds := &service.DispatchService{}
	ds.InitializeForTest(server.URL + "/")

	// Test
	ctx := context.Background()
	err := ds.HandleTokenExpired(ctx, 775)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestHandleTokenExpired_ErrorResponse tests error response
func TestHandleTokenExpired_ErrorResponse(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   "Account not found",
			"message": "The specified account does not exist",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound) // 404
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create service instance directly for testing
	ds := &service.DispatchService{}
	ds.InitializeForTest(server.URL + "/")

	// Test
	ctx := context.Background()
	err := ds.HandleTokenExpired(ctx, 999)

	// Assertions
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// TestGetAccountByUserId_NoURL tests error when URL is not configured
func TestGetAccountByUserId_NoURL(t *testing.T) {
	// Unset environment variable
	os.Unsetenv("ACCOUNT_MANAGER_URL")

	// Create a new service instance with empty URL
	ds := &service.DispatchService{}
	ds.InitializeForTest("")

	// Test
	ctx := context.Background()
	account, err := ds.GetAccountByUserId(ctx, 80)

	// Assertions
	if err == nil {
		t.Fatal("Expected error when URL is not configured, got nil")
	}
	if account != nil {
		t.Error("Expected account to be nil on error")
	}
}
