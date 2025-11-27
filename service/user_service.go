package service

import (
	"context"
	"encoding/json"
	"nursor-envoy-rpc/helper"
	"nursor-envoy-rpc/models"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// UserService manages user-related operations with Redis caching and token validation.
type UserService struct {
	defaultRedis                *redis.Client
	db                          *gorm.DB
	userCachePrefix             string
	userCachePrefixID           string
	userSubscriptionCachePrefix string
	initialized                 bool
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

	us.userCachePrefix = "nursor-rpc:user_cache:innertoken:"
	us.userCachePrefixID = "nursor-rpc:user_cache:id"
	us.userSubscriptionCachePrefix = "nursor-rpc:user_subscription_cache:"
	us.initialized = true
}

func (us *UserService) GetUserByInnerToken(ctx context.Context, innerToken string) (*models.User, error) {
	var user models.User
	usercache := us.defaultRedis.Get(ctx, us.userCachePrefix+innerToken)
	if usercache != nil {
		cacheBytes, err := usercache.Bytes()
		if err == nil {
			err := json.Unmarshal(cacheBytes, &user)
			if err != nil {
				err := us.db.WithContext(ctx).Where("inner_token = ?", innerToken).First(&user).Error
				if err != nil {
					return nil, err
				}
				us.defaultRedis.Set(ctx, us.userCachePrefix+innerToken, cacheBytes, 5*time.Minute)
			}
		}
	}
	if user.ID == 0 {
		err := us.db.WithContext(ctx).Where("inner_token = ?", innerToken).First(&user).Error
		if err != nil {
			return nil, err
		}
		cacheBytes, _ := json.Marshal(user)
		us.defaultRedis.Set(ctx, us.userCachePrefix+innerToken, cacheBytes, 5*time.Minute)
	}

	return &user, nil
}
