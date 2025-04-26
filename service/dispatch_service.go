package service

import (
	"context"
	"nursor-envoy-rpc/helper"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// DispatchService manages token dispatching and request recording.
type DispatchService struct {
	redisDispatcher *helper.RedisOperator
	tokenPersistent *helper.TokenPersistent
	redis           *redis.Client
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
	ds.redis = helper.GetNewRedis() // Assume this function exists
	ds.redisDispatcher = helper.GetInstanceRedisOperator()
	ds.tokenPersistent = helper.GetTPInstance()
	ds.initialized = true
}

// DispatchTokenForNewUser assigns a token to a new user.
func (ds *DispatchService) DispatchTokenForNewUser(ctx context.Context, userID string) (string, error) {
	tokenID, err := ds.redisDispatcher.HandleNewUser(ctx, userID)
	if err != nil {
		logrus.Errorf("Error handling new user %s: %v", userID, err)
		return "", err
	}
	if tokenID == "" {
		logrus.Error("No available cursor found")
		return "", nil
	}
	return tokenID, nil
}

// GetTokenByUserID retrieves the token associated with a user ID.
func (ds *DispatchService) GetTokenByUserID(ctx context.Context, userID string) (string, error) {
	tokenID, err := ds.redisDispatcher.GetTokenID(ctx, userID)
	if err != nil {
		logrus.Errorf("Error retrieving token for user %s: %v", userID, err)
		return "", err
	}
	return tokenID, nil
}

// RecordNewReq records a new request, including URL and token usage.
func (ds *DispatchService) RecordNewReq(ctx context.Context, userID, url string) (string, error) {
	// Record URL access
	_, err := ds.redisDispatcher.AddURLRecords(ctx, userID, url, 1)
	if err != nil {
		logrus.Errorf("Error recording URL for user %s: %v", userID, err)
		// Continue to increment token usage even if URL recording fails
	}

	// Increment token usage
	_, err = ds.redisDispatcher.IncrementTokenUsage(ctx, userID, 1)
	if err != nil {
		logrus.Errorf("Error incrementing token usage for user %s: %v", userID, err)
		// Continue to return token ID even if increment fails
	}

	// Get token ID
	tokenID, err := ds.redisDispatcher.GetTokenID(ctx, userID)
	if err != nil {
		logrus.Errorf("Error retrieving token for user %s: %v", userID, err)
		return "", err
	}
	return tokenID, nil
}

// ResetInstance resets the singleton instance (mainly for testing).
func ResetDispatchServiceInstance() {
	dispatchOnce = sync.Once{}
	dispatchInstance = nil
	logrus.Info("DispatchService singleton has been reset")
}
