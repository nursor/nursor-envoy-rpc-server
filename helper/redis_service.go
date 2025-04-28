package helper

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// RedisOperator is a singleton for managing Redis operations related to tokens and users.
type RedisOperator struct {
	redis            *redis.Client
	dbConnector      *TokenPersistent
	appName          string
	userTokenPrefix  string
	tokenUsersPrefix string
	tokenUsagePrefix string
	userUsagePrefix  string
	tokenListKey     string
	initialized      bool
}

// singleton redisOperatorInstance
var redisOperatorInstance *RedisOperator
var redisOnce sync.Once

// GetInstanceRedisOperator returns the singleton instance of RedisOperator.
func GetInstanceRedisOperator() *RedisOperator {
	redisOnce.Do(func() {
		redisClient := GetNewRedis()
		redisOperatorInstance = &RedisOperator{}
		redisOperatorInstance.initialize(redisClient)
	})
	return redisOperatorInstance
}

// initialize sets up the RedisOperator with the provided Redis client.
func (ro *RedisOperator) initialize(redisClient *redis.Client) {
	if ro.initialized {
		return
	}

	if redisClient == nil {
		redisClient = GetNewRedis() // Assume this function exists to get a new Redis client
	}

	ro.redis = redisClient
	ro.dbConnector = GetTPInstance()
	ro.appName = "nursor:dispatcher"
	ro.userTokenPrefix = fmt.Sprintf("%s:user_token:", ro.appName)
	ro.tokenUsersPrefix = fmt.Sprintf("%s:token_users:", ro.appName)
	ro.tokenUsagePrefix = fmt.Sprintf("%s:token_usage:", ro.appName)
	ro.userUsagePrefix = fmt.Sprintf("%s:user_usage:", ro.appName)
	ro.tokenListKey = fmt.Sprintf("%s:available_tokens", ro.appName)
	ro.initialized = true

	logrus.Info("RedisOperator singleton initialized")
}

// ResetInstance resets the singleton instance (mainly for testing).
func ResetRedisInstance() {
	redisOnce = sync.Once{}
	redisOperatorInstance = nil
	logrus.Info("RedisOperator singleton has been reset")
}

// BindingUserAndToken binds a user to a token.
func (ro *RedisOperator) BindingUserAndToken(ctx context.Context, userID, tokenID string) error {
	userKey := ro.userTokenPrefix + userID
	if err := ro.redis.Set(ctx, userKey, tokenID, 0).Err(); err != nil {
		return err
	}

	tokenUsersKey := ro.tokenUsersPrefix + tokenID
	if err := ro.redis.SAdd(ctx, tokenUsersKey, userID).Err(); err != nil {
		return err
	}

	return nil
}

// GetTokenIdByUserId retrieves the token bound to a user.
func (ro *RedisOperator) GetTokenIdByUserId(ctx context.Context, userID string) (string, error) {
	userKey := ro.userTokenPrefix + userID
	tokenID, err := ro.redis.Get(ctx, userKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return tokenID, nil
}

// BindUserAndToken sets a token for a user, removing old bindings if necessary.
func (ro *RedisOperator) BindUserAndToken(ctx context.Context, userID, tokenID string) error {
	oldToken, err := ro.GetTokenIdByUserId(ctx, userID)
	if err != nil {
		return err
	}

	if oldToken != "" {
		oldTokenUsersKey := ro.tokenUsersPrefix + oldToken
		if err := ro.redis.SRem(ctx, oldTokenUsersKey, userID).Err(); err != nil {
			return err
		}
	}

	return ro.BindingUserAndToken(ctx, userID, tokenID)
}

// DeleteToken removes a user's token binding.
func (ro *RedisOperator) DeleteToken(ctx context.Context, userID string) (int64, error) {
	token, err := ro.GetTokenIdByUserId(ctx, userID)
	if err != nil {
		return 0, err
	}
	if token == "" {
		return 0, nil
	}

	tokenUsersKey := ro.tokenUsersPrefix + token
	if err := ro.redis.SRem(ctx, tokenUsersKey, userID).Err(); err != nil {
		return 0, err
	}

	userKey := ro.userTokenPrefix + userID
	return ro.redis.Del(ctx, userKey).Result()
}

// IsTokenUnused checks if a token is unused.
func (ro *RedisOperator) IsTokenUnused(ctx context.Context, tokenID string) (bool, error) {
	tokenUsersKey := ro.tokenUsersPrefix + tokenID
	count, err := ro.redis.SCard(ctx, tokenUsersKey).Result()
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (ro *RedisOperator) GetMinBindingToken(ctx context.Context) (string, error) {
	allTokens, err := ro.GetAvailableTokens(ctx)
	if err != nil {
		return "", err
	}
	minUsersCount := int64(^uint64(0) >> 1) // Max int64
	var selectedToken string
	for _, token := range allTokens {
		tokenUsersKey := ro.tokenUsersPrefix + token
		usersCount, err := ro.redis.SCard(ctx, tokenUsersKey).Result()
		if err != nil {
			return "", err
		}
		if usersCount < minUsersCount {
			minUsersCount = usersCount
			selectedToken = token
		}
	}

	if selectedToken == "" {
		logrus.Error("No available tokens, please replenish")
		return "", nil
	}

	return selectedToken, nil

}

// HandleUserLeave cleans up records when a user leaves.
func (ro *RedisOperator) HandleUserLeave(ctx context.Context, userID string) error {
	_, err := ro.DeleteToken(ctx, userID)
	return err
}

func (ro *RedisOperator) GetAvailableTokens(ctx context.Context) ([]string, error) {
	return ro.redis.SMembers(ctx, ro.tokenListKey).Result()
}

// AssignNewToken assigns a token with the least bound users.
func (ro *RedisOperator) AssignNewToken(ctx context.Context) (string, error) {
	allTokens, err := ro.GetAvailableTokens(ctx)
	if err != nil {
		return "", err
	}
	if len(allTokens) == 0 {
		// TODO: 没有可用token，需要补充
		logrus.Error("No available tokens, please replenish")
		return "", nil
	}

	var selectedToken string
	minUsersCount := int64(^uint64(0) >> 1) // Max int64

	for _, token := range allTokens {
		tokenUsersKey := ro.tokenUsersPrefix + token
		usersCount, err := ro.redis.SCard(ctx, tokenUsersKey).Result()
		if err != nil {
			return "", err
		}
		if usersCount < minUsersCount {
			minUsersCount = usersCount
			selectedToken = token
		}
	}

	if selectedToken == "" {
		logrus.Error("No available tokens, please replenish")
		return "", nil
	}

	return selectedToken, nil
}

// IncrementTokenUsage increments token and user usage counts.
func (ro *RedisOperator) IncrementTokenUsage(ctx context.Context, tokenID string, count int64) (bool, error) {
	tokenUsageKey := ro.tokenUsagePrefix + tokenID
	if err := ro.redis.IncrBy(ctx, tokenUsageKey, count).Err(); err != nil {
		return false, err
	}

	userUsageKey := ro.userUsagePrefix + tokenID
	if err := ro.redis.IncrBy(ctx, userUsageKey, count).Err(); err != nil {
		return false, err
	}

	return true, nil
}

// GetTokenInfo retrieves detailed information about a token.
func (ro *RedisOperator) GetTokenInfo(ctx context.Context, tokenID string) (map[string]interface{}, error) {
	if tokenID == "" {
		return nil, nil
	}

	tokenUsersKey := ro.tokenUsersPrefix + tokenID
	tokenUsageKey := ro.tokenUsagePrefix + tokenID

	isAvailable, err := ro.redis.SIsMember(ctx, ro.tokenListKey, tokenID).Result()
	if err != nil {
		return nil, err
	}

	boundUsers, err := ro.redis.SMembers(ctx, tokenUsersKey).Result()
	if err != nil {
		return nil, err
	}
	boundUsersCount := len(boundUsers)

	usageCountStr, err := ro.redis.Get(ctx, tokenUsageKey).Result()
	if err == redis.Nil {
		usageCountStr = "0"
	} else if err != nil {
		return nil, err
	}
	usageCount, _ := strconv.ParseInt(usageCountStr, 10, 64)

	userUsageDetails := make(map[string]map[string]interface{})
	for _, userID := range boundUsers {
		userUsageKey := ro.userUsagePrefix + userID
		userUsageStr, err := ro.redis.Get(ctx, userUsageKey).Result()
		if err == redis.Nil {
			userUsageStr = "0"
		} else if err != nil {
			return nil, err
		}
		userUsage, _ := strconv.ParseInt(userUsageStr, 10, 64)

		userModelUsageKey := ro.userUsagePrefix + userID + ":models"
		userModelUsage, err := ro.redis.HGetAll(ctx, userModelUsageKey).Result()
		if err != nil {
			return nil, err
		}
		modelUsages := make(map[string]int64)
		for k, v := range userModelUsage {
			count, _ := strconv.ParseInt(v, 10, 64)
			modelUsages[k] = count
		}

		userUsageDetails[userID] = map[string]interface{}{
			"count":        userUsage,
			"model_usages": modelUsages,
		}
	}

	createdAt, err := ro.redis.Get(ctx, ro.tokenUsersPrefix+tokenID+":created_at").Result()
	if err == redis.Nil {
		createdAt = ""
	} else if err != nil {
		return nil, err
	}

	lastUsedAt, err := ro.redis.Get(ctx, ro.tokenUsersPrefix+tokenID+":last_used").Result()
	if err == redis.Nil {
		lastUsedAt = ""
	} else if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"token":              tokenID,
		"is_available":       isAvailable,
		"bound_users_count":  boundUsersCount,
		"bound_users":        boundUsers,
		"usage_count":        usageCount,
		"user_usage_details": userUsageDetails,
		"created_at":         createdAt,
		"last_used_at":       lastUsedAt,
	}, nil
}

// HandleTokenExpiration handles token expiration and replacement.
func (ro *RedisOperator) HandleTokenExpiration(ctx context.Context, expiredTokenID string) (string, error) {
	tokenInfo, err := ro.GetTokenInfo(ctx, expiredTokenID)
	if err != nil {
		return "", err
	}
	if tokenInfo != nil {
		if _, err := ro.dbConnector.SaveTokenData(ctx, tokenInfo); err != nil {
			return "", err
		}
	}

	newTokenID, err := ro.dbConnector.GetAvailableTokenIdFromDB(ctx, 1)
	if err != nil || len(newTokenID) == 0 {
		return "", err
	}

	if err := ro.ReplaceOldToken(ctx, expiredTokenID, *newTokenID[0].CursorID); err != nil {
		return "", err
	}

	return *newTokenID[0].CursorID, nil
}

// AddAvailableToken adds a token to the available tokens list.
func (ro *RedisOperator) AddAvailableToken(ctx context.Context, tokenID string) (int64, error) {
	return ro.redis.SAdd(ctx, ro.tokenListKey, tokenID).Result()
}

// ReplaceOldToken replaces an old token with a new one, migrating users.
func (ro *RedisOperator) ReplaceOldToken(ctx context.Context, oldTokenID, newTokenID string) error {
	tokenInfo, err := ro.GetTokenInfo(ctx, oldTokenID)
	if err != nil || tokenInfo == nil {
		return err
	}

	if _, err := ro.redis.SRem(ctx, ro.tokenListKey, oldTokenID).Result(); err != nil {
		return err
	}

	boundUsers := tokenInfo["bound_users"].([]string)
	if len(boundUsers) == 0 {
		return nil
	}

	if _, err := ro.redis.SAdd(ctx, ro.tokenListKey, newTokenID).Result(); err != nil {
		return err
	}

	for _, userID := range boundUsers {
		userKey := ro.userTokenPrefix + userID
		if err := ro.redis.Set(ctx, userKey, newTokenID, 0).Err(); err != nil {
			return err
		}
	}

	newTokenUsersKey := ro.tokenUsersPrefix + newTokenID
	if len(boundUsers) > 0 {
		args := make([]interface{}, len(boundUsers))
		for i, userID := range boundUsers {
			args[i] = userID
		}
		if err := ro.redis.SAdd(ctx, newTokenUsersKey, args...).Err(); err != nil {
			return err
		}
	}

	newTokenUsageKey := ro.tokenUsagePrefix + newTokenID
	if err := ro.redis.Set(ctx, newTokenUsageKey, 0, 0).Err(); err != nil {
		return err
	}

	oldTokenUsersKey := ro.tokenUsersPrefix + oldTokenID
	oldTokenUsageKey := ro.tokenUsagePrefix + oldTokenID
	if _, err := ro.redis.Del(ctx, oldTokenUsersKey, oldTokenUsageKey).Result(); err != nil {
		return err
	}

	logrus.Infof("Successfully migrated %d users from %s to %s", len(boundUsers), oldTokenID, newTokenID)
	return nil
}

// GetUserUsage retrieves a user's usage count.
func (ro *RedisOperator) GetUserUsage(ctx context.Context, userID string) (int64, error) {
	userUsageKey := ro.userUsagePrefix + userID
	usage, err := ro.redis.Get(ctx, userUsageKey).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(usage, 10, 64)
}

// GetTokenUsage retrieves a token's usage count.
func (ro *RedisOperator) GetTokenUsage(ctx context.Context, tokenID string) (int64, error) {
	tokenUsageKey := ro.tokenUsagePrefix + tokenID
	usage, err := ro.redis.Get(ctx, tokenUsageKey).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(usage, 10, 64)
}

// AddURLRecords records a user's URL access.
func (ro *RedisOperator) AddURLRecords(ctx context.Context, userID, url string, count int64) (bool, error) {
	token, err := ro.GetTokenIdByUserId(ctx, userID)
	if err != nil || token == "" {
		logrus.Warnf("User %s has no bound token, cannot record URL access", userID)
		return false, err
	}

	userURLKey := ro.userUsagePrefix + userID + ":urls"
	currentCount, err := ro.redis.HGet(ctx, userURLKey, url).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}
	newCount := count
	if currentCount != "" {
		countVal, _ := strconv.ParseInt(currentCount, 10, 64)
		newCount = countVal + count
	}
	if err := ro.redis.HSet(ctx, userURLKey, url, newCount).Err(); err != nil {
		return false, err
	}

	tokenURLKey := ro.tokenUsagePrefix + token + ":urls"
	tokenURLCount, err := ro.redis.HGet(ctx, tokenURLKey, url).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}
	tokenNewCount := count
	if tokenURLCount != "" {
		countVal, _ := strconv.ParseInt(tokenURLCount, 10, 64)
		tokenNewCount = countVal + count
	}
	if err := ro.redis.HSet(ctx, tokenURLKey, url, tokenNewCount).Err(); err != nil {
		return false, err
	}

	expireDuration := 3 * 24 * time.Hour
	if err := ro.redis.Expire(ctx, userURLKey, expireDuration).Err(); err != nil {
		return false, err
	}
	if err := ro.redis.Expire(ctx, tokenURLKey, expireDuration).Err(); err != nil {
		return false, err
	}

	currentTime := time.Now().Format(time.RFC3339)
	if err := ro.redis.Set(ctx, ro.userUsagePrefix+userID+":last_url_access", currentTime, 0).Err(); err != nil {
		return false, err
	}
	if err := ro.redis.Set(ctx, ro.tokenUsersPrefix+token+":last_used", currentTime, 0).Err(); err != nil {
		return false, err
	}

	logrus.Debugf("Recorded URL access: user_id=%s, url=%s, count=%d, total=%d", userID, url, count, newCount)
	return true, nil
}

// GetUserURLRecords retrieves a user's URL access records.
func (ro *RedisOperator) GetUserURLRecords(ctx context.Context, userID string) (map[string]int64, error) {
	userURLKey := ro.userUsagePrefix + userID + ":urls"
	urlRecords, err := ro.redis.HGetAll(ctx, userURLKey).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for url, count := range urlRecords {
		countVal, _ := strconv.ParseInt(count, 10, 64)
		result[url] = countVal
	}

	return result, nil
}

// DeleteAuthToken deletes a user's auth token.
func (ro *RedisOperator) DeleteAuthToken(ctx context.Context, authToken string) (int64, error) {
	userKey := ro.userTokenPrefix + "auth_token:" + authToken
	return ro.redis.Del(ctx, userKey).Result()
}

// AddModelUsage increments model usage counts for a user and token.
func (ro *RedisOperator) AddModelUsage(ctx context.Context, userID, modelName string, count int64) (bool, error) {
	token, err := ro.GetTokenIdByUserId(ctx, userID)
	if err != nil || token == "" {
		return false, err
	}

	userUsageKey := ro.userUsagePrefix + userID + ":models"
	currentCount, err := ro.redis.HGet(ctx, userUsageKey, modelName).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}
	newCount := count
	if currentCount != "" {
		countVal, _ := strconv.ParseInt(currentCount, 10, 64)
		newCount = countVal + count
	}
	if err := ro.redis.HSet(ctx, userUsageKey, modelName, newCount).Err(); err != nil {
		return false, err
	}

	tokenUsageKey := ro.tokenUsagePrefix + token + ":models"
	tokenCount, err := ro.redis.HGet(ctx, tokenUsageKey, modelName).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}
	tokenNewCount := count
	if tokenCount != "" {
		countVal, _ := strconv.ParseInt(tokenCount, 10, 64)
		tokenNewCount = countVal + count
	}
	if err := ro.redis.HSet(ctx, tokenUsageKey, modelName, tokenNewCount).Err(); err != nil {
		return false, err
	}

	return true, nil
}

// GetNewRedis is a placeholder for getting a new Redis client (implement as needed).
func GetNewRedis() *redis.Client {
	// Implement Redis client initialization
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "172.16.238.2:30706"
	}
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	if redisDB == 0 {
		redisDB = 12
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	return redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		DB:       redisDB,
		Password: redisPassword,
	})
}

// NewTokenPersistent is a placeholder for creating a TokenPersistent implementation.
func NewTokenPersistent() *TokenPersistent {
	// Implement TokenPersistent creation
	return nil
}
