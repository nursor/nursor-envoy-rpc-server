package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nursor-envoy-rpc/models"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// UserService manages user-related operations with Redis caching and token validation.
type UserService struct {
	defaultRedis      *redis.Client
	db                *gorm.DB
	dispatcherService DispatchService
	userCachePrefix   string
	userCachePrefixID string
	initialized       bool
}

// singleton instance
var userInstance *UserService
var userOnce sync.Once

// GetUserServiceInstance returns the singleton instance of UserService.
func GetUserServiceInstance() *UserService {
	userOnce.Do(func() {

		db := getNewDB()
		redisClient := getNewRedis()
		userInstance = &UserService{}
		userInstance.initialize(db, redisClient)
	})
	return userInstance
}

// initialize sets up the UserService with the provided database and Redis client.
func (us *UserService) initialize(db *gorm.DB, redisClient *redis.Client) {
	if us.initialized {
		return
	}
	if redisClient == nil {
		redisClient = getNewRedis() // Assume this function exists
	}
	if db == nil {
		db = getNewDB() // Assume this function exists
	}
	us.defaultRedis = redisClient
	us.db = db
	us.dispatcherService = *GetDispatchInstance()
	us.userCachePrefix = "user_cache:token"
	us.userCachePrefixID = "user_cache:id"
	us.initialized = true
}

// ParseRequestToken validates the token in an HTTP flow and sets user info.
func (us *UserService) ParseRequestToken(ctx context.Context, authrozationValue string) (bool, error) {
	parts := strings.Split(authrozationValue, " ")
	if len(parts) < 2 {
		return true, nil
	}
	fakeInnerToken := parts[1]

	if fakeInnerToken == "" {
		return true, nil
	}

	// Extract inner token
	innerTokenParts := strings.Split(fakeInnerToken, ".")
	innerToken := innerTokenParts[len(innerTokenParts)-1]

	// Check token availability
	isAvailable, err := us.IsUserAvailable(ctx, innerToken)
	if err != nil {
		return false, err
	}
	if !isAvailable {
		logrus.Infof("Invalid or expired token: %s", innerToken)
		return false, nil
	}

	// Get user information
	userInfo, err := us.GetUserByInnerToken(ctx, innerToken)
	if err != nil {
		return false, err
	}
	if userInfo == nil {
		return false, errors.New("user not found")
	}
	return true, nil
}

// GetUserByToken retrieves user information by access token, using Redis cache.
func (us *UserService) GetUserByToken(ctx context.Context, userToken string) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("%s:%s", us.userCachePrefix, userToken)
	userInfoJSON, err := us.defaultRedis.Get(ctx, cacheKey).Result()
	if err == nil && userInfoJSON != "" {
		var userInfo map[string]interface{}
		if err := json.Unmarshal([]byte(userInfoJSON), &userInfo); err != nil {
			logrus.Errorf("Error unmarshaling cached user info: %v", err)
			return nil, err
		}
		return userInfo, nil
	}
	if err != redis.Nil {
		logrus.Errorf("Error accessing Redis: %v", err)
	}

	var user models.User
	err = us.db.WithContext(ctx).Where("access_token = ?", userToken).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		logrus.Errorf("Error querying user by token: %v", err)
		return nil, err
	}

	userInfo := map[string]interface{}{
		"id":           user.ID,
		"access_token": user.AccessToken,
		"inner_token":  user.InnerToken,
		"usage":        user.Usage,
		"limit":        user.Limit,
		"expired_at":   user.ExpiredAt,
	}
	userInfoJSONBytes, err := json.Marshal(userInfo)
	if err != nil {
		logrus.Errorf("Error marshaling user info: %v", err)
		return userInfo, nil // Return user info anyway
	}

	err = us.defaultRedis.Set(ctx, cacheKey, string(userInfoJSONBytes), 30*time.Minute).Err()
	if err != nil {
		logrus.Errorf("Error caching user info: %v", err)
	}

	return userInfo, nil
}

// GetUserByID retrieves user information by user ID, using Redis cache.
func (us *UserService) GetUserByID(ctx context.Context, userID int) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("%s:%d", us.userCachePrefixID, userID)
	userInfoJSON, err := us.defaultRedis.Get(ctx, cacheKey).Result()
	if err == nil && userInfoJSON != "" {
		var userInfo map[string]interface{}
		if err := json.Unmarshal([]byte(userInfoJSON), &userInfo); err != nil {
			logrus.Errorf("Error unmarshaling cached user info: %v", err)
			return nil, err
		}
		return userInfo, nil
	}
	if err != redis.Nil {
		logrus.Errorf("Error accessing Redis: %v", err)
	}

	var user models.User
	err = us.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		logrus.Errorf("Error querying user by ID: %v", err)
		return nil, err
	}

	userInfo := map[string]interface{}{
		"id":           user.ID,
		"access_token": user.AccessToken,
		"inner_token":  user.InnerToken,
		"usage":        user.Usage,
		"limit":        user.Limit,
		"expired_at":   user.ExpiredAt,
	}
	userInfoJSONBytes, err := json.Marshal(userInfo)
	if err != nil {
		logrus.Errorf("Error marshaling user info: %v", err)
		return userInfo, nil // Return user info anyway
	}

	err = us.defaultRedis.Set(ctx, cacheKey, string(userInfoJSONBytes), 30*time.Minute).Err()
	if err != nil {
		logrus.Errorf("Error caching user info: %v", err)
	}

	return userInfo, nil
}

// IsUserAvailable checks if a user is available based on their inner token.
func (us *UserService) IsUserAvailable(ctx context.Context, innerToken string) (bool, error) {
	user, err := us.GetUserByInnerToken(ctx, innerToken)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}

	usageFloat, _ := user["usage"].(float64)
	usage := int(usageFloat)
	limitFloat, _ := user["limit"].(float64)
	limit := int(limitFloat)
	if usage > limit {
		return false, nil
	}

	expiredAtStr, ok := user["expired_at"].(string)
	if !ok {
		return false, fmt.Errorf("invalid expired_at format")
	}
	expiredAt, err := time.Parse(time.RFC3339, expiredAtStr)
	if err != nil {
		logrus.Errorf("Error parsing expired_at: %v", err)
		return false, err
	}
	if expiredAt.Before(time.Now().UTC()) {
		return false, nil
	}

	return true, nil
}

// GetUserByInnerToken retrieves user information by inner token, using Redis cache.
func (us *UserService) GetUserByInnerToken(ctx context.Context, innerToken string) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("%s:%s", us.userCachePrefix, innerToken)
	userInfoJSON, err := us.defaultRedis.Get(ctx, cacheKey).Result()
	if err == nil && userInfoJSON != "" {
		var userInfo map[string]interface{}
		if err := json.Unmarshal([]byte(userInfoJSON), &userInfo); err != nil {
			logrus.Errorf("Error unmarshaling cached user info: %v", err)
			return nil, err
		}
		return userInfo, nil
	}
	if err != redis.Nil {
		logrus.Errorf("Error accessing Redis: %v", err)
	}

	var user models.User
	err = us.db.WithContext(ctx).Where("inner_token = ?", innerToken).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		logrus.Errorf("Error querying user by inner token: %v", err)
		return nil, err
	}

	userInfo := map[string]interface{}{
		"id":           user.ID,
		"access_token": user.AccessToken,
		"inner_token":  user.InnerToken,
		"usage":        user.Usage,
		"limit":        user.Limit,
		"expired_at":   user.ExpiredAt,
	}
	userInfoJSONBytes, err := json.Marshal(userInfo)
	if err != nil {
		logrus.Errorf("Error marshaling user info: %v", err)
		return userInfo, nil // Return user info anyway
	}

	err = us.defaultRedis.Set(ctx, cacheKey, string(userInfoJSONBytes), 30*time.Minute).Err()
	if err != nil {
		logrus.Errorf("Error caching user info: %v", err)
	}

	return userInfo, nil
}

// ResetInstance resets the singleton instance (mainly for testing).
func ResetUserServiceInstance() {
	userOnce = sync.Once{}
	userInstance = nil
	logrus.Info("UserService singleton has been reset")
}
