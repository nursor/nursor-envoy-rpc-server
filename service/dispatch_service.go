package service

import (
	"context"
	"nursor-envoy-rpc/helper"
	"nursor-envoy-rpc/models"
	"sync"
	"time"

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

// AssignNewTokenForUser assigns a token with the least bound users.
func (ds *DispatchService) AssignNewTokenForUser(ctx context.Context) (string, error) {
	allTokens, err := ds.redisDispatcher.GetAvailableTokens(ctx)
	if err != nil {
		return "", err
	}
	if len(allTokens) == 0 {
		// TODO: 没有可用token，需要补充
		logrus.Error("No available tokens, please replenish")
		return "", nil
	}

	minUsageToken, err := ds.redisDispatcher.GetMinBindingToken(ctx)
	if err != nil {
		return "", err
	}
	return minUsageToken, nil
}

// HandleNewUser processes a new user and assigns a token if needed.
func (ds *DispatchService) DispatchTokenForNewUser(ctx context.Context, userID string) (string, error) {
	newTokenID, err := ds.redisDispatcher.AssignNewToken(ctx)
	if err != nil || newTokenID == "" {
		logrus.Errorf("Failed to assign token to user %s: %v", userID, err)
		return "", err
	}

	if err := ds.redisDispatcher.BindUserAndToken(ctx, userID, newTokenID); err != nil {
		return "", err
	}
	return newTokenID, nil
}

// DispatchTokenForUser assigns a token to a new user.
func (ds *DispatchService) DispatchTokenForUser(ctx context.Context, userID string) (*models.Cursor, error) {
	tokenID, err := ds.redisDispatcher.GetTokenIdByUserId(ctx, userID)
	if err != nil {
		logrus.Errorf("Error retrieving token for user %s: %v", userID, err)
		return nil, err
	}
	if tokenID == "" {
		tokenID, err = ds.DispatchTokenForNewUser(ctx, userID)
		if err != nil {
			logrus.Errorf("Error handling new user %s: %v", userID, err)
			return nil, err
		}
	}
	if tokenID == "" {
		logrus.Error("No available cursor found")
		return nil, nil
	}
	// 从tokenId获取token
	token, err := ds.tokenPersistent.GetTokenByTokenId(ctx, tokenID)
	if err != nil {
		logrus.Errorf("Error retrieving token for user %s: %v", userID, err)
		return nil, err
	}
	return token, nil
}

func (ds *DispatchService) IncrTokenUsage(ctx context.Context, tokenID string) error {
	_, err := ds.redisDispatcher.IncrementTokenUsage(ctx, tokenID, 1)
	return err
}

// ResetInstance resets the singleton instance (mainly for testing).
func ResetDispatchServiceInstance() {
	dispatchOnce = sync.Once{}
	dispatchInstance = nil
	logrus.Info("DispatchService singleton has been reset")
}

func KeepTokenQueueAvailable(ctx context.Context) {
	ds := GetDispatchInstance()
	allTokens, err := ds.redisDispatcher.GetAvailableTokens(ctx)
	if err != nil {
		return
	}
	for _, token := range allTokens {
		tokenUsage, err := ds.redisDispatcher.GetTokenUsage(ctx, token)
		if err != nil {
			return
		}
		if tokenUsage > 150 {
			ds.redisDispatcher.DeleteToken(ctx, token)
		}
	}
	loopCnt := 10 - len(allTokens)

	tokenId, err := ds.tokenPersistent.GetAvailableTokenIdFromDB(ctx, loopCnt)
	if err != nil {
		return
	}
	for _, token := range tokenId {
		ds.redisDispatcher.AddAvailableToken(ctx, *token.CursorID)
	}
}

func init() {
	go func() {
		for {
			KeepTokenQueueAvailable(context.Background())
			time.Sleep(10 * time.Second)
		}
	}()
}
