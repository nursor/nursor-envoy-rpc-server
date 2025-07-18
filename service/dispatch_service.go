package service

import (
	"context"
	"nursor-envoy-rpc/helper"
	"nursor-envoy-rpc/models"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// DispatchService manages token dispatching and request recording.
type DispatchService struct {
	redisDispatcher *helper.RedisOperator
	tokenPersistent *helper.TokenPersistent
	userService     *UserService
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
	ds.userService = GetUserServiceInstance()
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
func (ds *DispatchService) DispatchTokenForNewUser(ctx context.Context, user *models.User) (string, error) {
	newTokenID, err := ds.redisDispatcher.AssignNewToken(ctx)
	if err != nil || newTokenID == "" {
		logrus.Errorf("Failed to assign token to user %d: %v", user.ID, err)
		return "", err
	}

	if err := ds.redisDispatcher.BindUserAndToken(ctx, strconv.Itoa(user.ID), newTokenID); err != nil {
		return "", err
	}
	if _, err := ds.redisDispatcher.InitTokenUsage(ctx, newTokenID); err != nil {
		return "", err
	}
	if err := ds.redisDispatcher.SetUserUsage(ctx, strconv.Itoa(user.ID), int64(user.Usage)); err != nil {
		return "", err
	}

	return newTokenID, nil
}

// DispatchTokenForUser assigns a token to a new user.
func (ds *DispatchService) DispatchTokenForUser(ctx context.Context, user *models.User) (*models.Cursor, error) {
	userID := strconv.Itoa(user.ID)
	tokenID, err := ds.redisDispatcher.GetTokenIdByUserId(ctx, userID)
	if err != nil {
		logrus.Errorf("Error retrieving token for user %s: %v", userID, err)
		return nil, err
	}
	if tokenID == "" {
		tokenID, err = ds.DispatchTokenForNewUser(ctx, user)
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

func (ds *DispatchService) IncrTokenUsage(ctx context.Context, innerToken string) error {
	// 获取用户信息
	user, err := ds.userService.GetUserByInnerToken(ctx, innerToken)
	if err != nil {
		return err
	}
	// 获取tokenID
	tokenID, err := ds.redisDispatcher.GetTokenIdByUserId(ctx, strconv.Itoa(user.ID))
	if err != nil {
		return err
	}
	// 增加token使用次数
	_, err = ds.redisDispatcher.IncrementTokenUsage(ctx, tokenID, 1)
	if err != nil {
		return err
	}
	// 增加用户使用次数
	_, err = ds.redisDispatcher.IncrementUserUsage(ctx, strconv.Itoa(user.ID), 1)
	if err != nil {
		return err
	}
	// 保存用户使用次数
	go func() {
		user.Usage++
		ds.userService.db.WithContext(ctx).Save(&user)
	}()
	logrus.Infof("IncrTokenUsage: user %d, tokenID %s, usage %d", user.ID, tokenID, user.Usage)
	return nil
}

func (ds *DispatchService) HandleTokenExpired(ctx context.Context, tokenID string) error {
	userIds, err := ds.redisDispatcher.GetUserIdByToken(ctx, tokenID)
	if err != nil {
		return err
	}
	for _, userId := range userIds {
		removedUsage, err := ds.redisDispatcher.RemoveCacheUserId(ctx, userId)
		if err != nil {
			continue
		}
		userIdInt, err := strconv.Atoi(userId)
		if err != nil {
			continue
		}
		user, err := ds.userService.GetUserByID(ctx, userIdInt)
		if err != nil {
			continue
		}
		if removedUsage > 0 {
			ds.userService.CompareAndSaveTokenUsage(ctx, user, int(removedUsage))
			ds.DispatchTokenForNewUser(ctx, user)
		}
	}
	ds.redisDispatcher.RemoveCachedToken(ctx, tokenID)
	return nil
}

func (ds *DispatchService) GetTokenIdByInnerToken(ctx context.Context, innerToken string) (string, error) {
	user, err := ds.userService.GetUserByInnerToken(ctx, innerToken)
	if err != nil {
		return "", err
	}
	// 获取tokenID
	tokenID, err := ds.redisDispatcher.GetTokenIdByUserId(ctx, strconv.Itoa(user.ID))
	if err != nil {
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

func KeepTokenQueueAvailable(ctx context.Context) {
	ds := GetDispatchInstance()
	tokenKeepSize := os.Getenv("TOKEN_KEEP_SIZE")
	if tokenKeepSize == "" {
		tokenKeepSize = "10"
	}
	tokenKeepSizeInt, err := strconv.Atoi(tokenKeepSize)
	if err != nil {
		return
	}
	allTokens, err := ds.redisDispatcher.GetAvailableTokens(ctx)
	if err != nil {
		return
	}
	for _, tokenID := range allTokens {
		tokenUsage, err := ds.redisDispatcher.GetTokenUsage(ctx, tokenID)
		if err != nil {
			return
		}
		if tokenUsage > 40 {
			ds.HandleTokenExpired(ctx, tokenID)
		}
	}
	tokenKeepSizeDiff := tokenKeepSizeInt - len(allTokens)

	tokenIds, err := ds.tokenPersistent.GetAvailableTokenIdFromDB(ctx, tokenKeepSizeDiff)
	if err != nil {
		return
	}
	for _, token := range tokenIds {
		ds.redisDispatcher.AddAvailableToken(ctx, *token.CursorID)
	}
}

func init() {
	go func() {
		for {
			// KeepTokenQueueAvailable(context.TODO())
			time.Sleep(10 * time.Second)
		}
	}()
}
