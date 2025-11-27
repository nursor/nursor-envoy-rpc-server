package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"nursor-envoy-rpc/models"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// DispatchService manages token dispatching and request recording.
type DispatchService struct {
	userService     *UserService
	accountMagerUrl string
	initialized     bool
}

// singleton instance
var dispatchInstance *DispatchService
var dispatchOnce sync.Once

// GetInstance returns the singleton instance of DispatchService.
func GetDispatchInstance() *DispatchService {
	dispatchOnce.Do(func() {
		dispatchInstance = &DispatchService{}
		dispatchInstance.initialize()
	})
	return dispatchInstance
}

// initialize sets up the DispatchService with Redis and token persistent dependencies.
func (ds *DispatchService) initialize() {
	if ds.initialized {
		return
	}
	ds.accountMagerUrl = os.Getenv("ACCOUNT_MANAGER_URL")
	if ds.accountMagerUrl == "" {
		ds.accountMagerUrl = "http://172.16.238.2:31219/"
	}

	ds.userService = GetUserServiceInstance()
	ds.initialized = true
}

// InitializeForTest initializes the service for testing purposes with a custom URL
func (ds *DispatchService) InitializeForTest(url string) {
	ds.accountMagerUrl = url
	ds.userService = GetUserServiceInstance()
	ds.initialized = true
}

// AcquireAccountRequest represents the request body for acquiring an account
type AcquireAccountRequest struct {
	UserID string `json:"userId"`
}

// AcquireAccountResponse represents the successful response from the account manager
type AcquireAccountResponse struct {
	Account models.AccountInfo `json:"account"`
	Reused  bool               `json:"reused"`
}

// AccountInfo represents the account information in the response

// AcquireAccountErrorResponse represents the error response from the account manager
type AcquireAccountErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// AssignNewTokenForUser acquires a new account for the user via HTTP request
func (ds *DispatchService) GetAccountByUserId(ctx context.Context, userID int) (*models.AccountInfo, error) {
	// Prepare request
	reqBody := AcquireAccountRequest{
		UserID: strconv.Itoa(userID),
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	url := ds.accountMagerUrl
	if url == "" {
		return nil, fmt.Errorf("account manager URL is not configured")
	}
	if url[len(url)-1] != '/' {
		url += "/"
	}
	url += "acquire"

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Send request
	logrus.Infof("Sending request to acquire account for user %d: %s", userID, url)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle error responses (402 or other error status codes)
	if resp.StatusCode == http.StatusPaymentRequired || resp.StatusCode >= 400 {
		var errorResp AcquireAccountErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("account manager returned error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("account manager error: %s - %s", errorResp.Error, errorResp.Message)
	}

	// Parse successful response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var accountResp AcquireAccountResponse
	if err := json.Unmarshal(body, &accountResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w, body: %s", err, string(body))
	}

	// Convert AccountInfo to models.Cursor
	logrus.Infof("Successfully acquired account for user %d: cursor_id=%s", userID, accountResp.Account.CursorID)
	return &accountResp.Account, nil
}

// IncrUsageRequest represents the request body for incrementing account usage
type IncrUsageRequest struct {
	AccountID int `json:"accountId"`
}

// IncrTokenUsage increments the usage count for an account via HTTP request
func (ds *DispatchService) IncrTokenUsage(ctx context.Context, AccountId int) error {
	// Prepare request
	reqBody := IncrUsageRequest{
		AccountID: AccountId,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	url := ds.accountMagerUrl
	if url == "" {
		return fmt.Errorf("account manager URL is not configured")
	}
	if url[len(url)-1] != '/' {
		url += "/"
	}
	url += "usage/inc"

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
	logrus.Infof("Sending request to increment usage for account %d: %s", AccountId, url)
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
		var errorResp AcquireAccountErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return fmt.Errorf("account manager returned error (status %d): %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("account manager error: %s - %s", errorResp.Error, errorResp.Message)
	}

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	logrus.Infof("Successfully incremented usage for account %d", AccountId)
	return nil
}

// HandleTokenExpired disables an expired account via HTTP request
func (ds *DispatchService) HandleTokenExpired(ctx context.Context, AccountId int) error {
	// Build URL with accountId in the path
	url := ds.accountMagerUrl
	if url == "" {
		return fmt.Errorf("account manager URL is not configured")
	}
	if url[len(url)-1] != '/' {
		url += "/"
	}
	url += fmt.Sprintf("account/%d/disable-with-check", AccountId)

	// Create HTTP request with empty body
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Send request
	logrus.Infof("Sending request to disable expired account %d: %s", AccountId, url)
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
		var errorResp AcquireAccountErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return fmt.Errorf("account manager returned error (status %d): %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("account manager error: %s - %s", errorResp.Error, errorResp.Message)
	}

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	logrus.Infof("Successfully disabled expired account %d", AccountId)
	return nil
}
