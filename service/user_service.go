package service

import (
	"context"
	"errors"
	"fmt"
	"nursor-envoy-rpc/helper"
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
	redisDispatcher   *helper.RedisOperator
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
		db := helper.GetNewDB()
		redisClient := helper.GetNewRedis()
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
		redisClient = helper.GetNewRedis() // Assume this function exists
	}
	if db == nil {
		db = helper.GetNewDB() // Assume this function exists
	}
	us.defaultRedis = redisClient
	us.db = db

	us.userCachePrefix = "user_cache:token"
	us.userCachePrefixID = "user_cache:id"
	us.redisDispatcher = helper.GetInstanceRedisOperator()
	us.initialized = true
}

// CheckAndGetUserFromInnerToken validates the token in an HTTP flow and sets user info.
func (us *UserService) CheckAndGetUserFromInnerToken(ctx context.Context, authrozationValue string) (*models.User, error) {
	parts := strings.Split(authrozationValue, " ")
	if len(parts) < 2 {
		return nil, errors.New("invalid authorization header")
	}
	fakeInnerToken := parts[1]

	if fakeInnerToken == "" {
		return nil, errors.New("empty token")
	}

	// Extract inner token
	innerTokenParts := strings.Split(fakeInnerToken, ".")
	innerToken := innerTokenParts[len(innerTokenParts)-1]

	// Check token availability
	isAvailable, err := us.IsUserAvailable(ctx, innerToken)
	if err != nil {
		return nil, err
	}
	if !isAvailable {
		logrus.Infof("Invalid or expired token: %s", innerToken)
		return nil, errors.New("invalid or expired token")
	}

	userInfo, err := us.GetUserByInnerTokenFromDB(ctx, innerToken)
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}

func (us *UserService) GetUserByInnerTokenFromDB(ctx context.Context, innerToken string) (*models.User, error) {
	var user models.User
	err := us.db.WithContext(ctx).Where("inner_token = ?", innerToken).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (us *UserService) GetUserByInnerToken(ctx context.Context, innerToken string) (*models.User, error) {
	var user models.User
	err := us.db.WithContext(ctx).Where("inner_token = ?", innerToken).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID retrieves user information by user ID, using Redis cache.
func (us *UserService) GetUserByID(ctx context.Context, userID int) (*models.User, error) {

	var user models.User
	err := us.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		logrus.Errorf("Error querying user by ID: %v", err)
		return nil, err
	}

	return &user, nil
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

	usage := user.Usage
	limit := user.Limit
	if usage > limit {
		return false, nil
	}

	expiredAt := user.ExpiredAt
	if expiredAt == nil {
		return false, fmt.Errorf("invalid expired_at format")
	}
	if expiredAt.Before(time.Now().UTC()) {
		return false, nil
	}
	return true, nil
}

func (us *UserService) IncrementTokenUsage(ctx context.Context, innerToken string) error {
	user, err := us.GetUserByInnerToken(ctx, innerToken)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	user.Usage++
	err = us.db.WithContext(ctx).Save(&user).Error
	if err != nil {
		return err
	}
	return nil
}

func (us *UserService) CompareAndSaveTokenUsage(ctx context.Context, user *models.User, usage int) error {
	if user == nil {
		return errors.New("user not found")
	}
	if user.Usage > usage {
		return nil
	}
	user.Usage = usage
	err := us.db.WithContext(ctx).Save(&user).Error
	if err != nil {
		return err
	}
	return nil
}

// ResetInstance resets the singleton instance (mainly for testing).
func ResetUserServiceInstance() {
	userOnce = sync.Once{}
	userInstance = nil
	logrus.Info("UserService singleton has been reset")
}
