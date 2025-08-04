package service

import (
	"context"
	"encoding/json"
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
	defaultRedis                *redis.Client
	db                          *gorm.DB
	redisDispatcher             *helper.RedisOperator
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

	us.userCachePrefix = "user_cache:innertoken:"
	us.userCachePrefixID = "user_cache:id"
	us.userSubscriptionCachePrefix = "user_subscription_cache:"
	us.redisDispatcher = helper.GetInstanceRedisOperator()
	us.initialized = true
}

// CheckAndGetUserFromInnerToken validates the token in an HTTP flow and sets user info.
func (us *UserService) CheckAndGetUserFromBindingtoken(ctx context.Context, authrozationValue string) (*models.User, error) {
	// Check token availability
	if strings.Contains(authrozationValue, "Bearer") {
		authrozationValue = strings.Replace(authrozationValue, "Bearer ", "", -1)
	}
	bindShip := models.UserCursorTokenBinding{}
	user, err := bindShip.FindUserByCursorToken(us.db, authrozationValue)
	if err != nil {
		return nil, err
	}

	isAvailable, err := us.IsUserAvailable(ctx, user)
	if err != nil {
		return nil, err
	}
	if !isAvailable {
		uspt, err := us.ActiveNewSubscriptionFromPending(ctx, int(user.ID))
		if err != nil {
			return nil, err
		}
		if uspt == nil {
			return nil, errors.New("no pending subscription")
		}
		return user, nil
	}

	return user, nil
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

	isAvailable, err := us.IsUserAvailable(ctx, &user)
	if err != nil {
		return nil, err
	}
	if !isAvailable {
		uspt, err := us.ActiveNewSubscriptionFromPending(ctx, int(user.ID))
		if err != nil {
			return nil, err
		}
		if uspt == nil {
			return nil, errors.New("no pending subscription")
		}
		return &user, nil
	}

	return &user, nil
}

// GetUserSubscriptionsByUserIDAndStatus retrieves user subscriptions with Redis caching
func (us *UserService) GetUserSubscriptionsByUserIDAndStatus(ctx context.Context, userID uint, status string) ([]models.UserSubscription, error) {
	cacheKey := us.userSubscriptionCachePrefix + fmt.Sprintf("%d:%s", userID, status)

	// Try to get from cache first
	cacheResult := us.defaultRedis.Get(ctx, cacheKey)
	var subscriptions []models.UserSubscription
	if cacheResult != nil {
		cacheBytes, err := cacheResult.Bytes()
		if err == nil {
			err := json.Unmarshal(cacheBytes, &subscriptions)
			if err == nil {
				return subscriptions, nil
			}
		}
	}

	// If not in cache or cache invalid, query from database
	uspt := models.UserSubscription{}
	subscriptions, err := uspt.FindUserSubscriptionsByUserIDAndStatus(us.db, userID, status)
	if err != nil {
		return nil, err
	}

	// Cache the result for 5 minutes
	cacheBytes, _ := json.Marshal(subscriptions)
	us.defaultRedis.Set(ctx, cacheKey, cacheBytes, 5*time.Minute)

	return subscriptions, nil
}

// ClearUserSubscriptionCache clears the cache for a specific user's subscriptions
func (us *UserService) ClearUserSubscriptionCache(ctx context.Context, userID uint) error {
	// Clear cache for all possible statuses
	statuses := []string{"active", "pending", "expired"}
	for _, status := range statuses {
		cacheKey := us.userSubscriptionCachePrefix + fmt.Sprintf("%d:%s", userID, status)
		us.defaultRedis.Del(ctx, cacheKey)
	}
	return nil
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
func (us *UserService) IsUserAvailable(ctx context.Context, user *models.User) (bool, error) {
	if user == nil {
		return false, nil
	}
	// 一般只有一个
	uspts, err := us.GetUserSubscriptionsByUserIDAndStatus(ctx, uint(user.ID), "active")
	if err != nil {
		return false, err
	}
	if len(uspts) == 0 {
		return false, nil
	}

	isAvailable := false
	for i := range uspts {
		if uspts[i].EndDate.Before(time.Now()) {
			uspts[i].Status = "expired"
			us.db.WithContext(ctx).Save(&uspts[i])
			continue
		}
		if uspts[i].UsedTraffic >= *uspts[i].Subscription.TrafficLimit {
			uspts[i].Status = "expired"
			us.db.WithContext(ctx).Save(&uspts[i])
			continue
		}
		if uspts[i].CursorAskUsage >= uspts[i].Subscription.CursorAskCount {
			uspts[i].Status = "expired"
			us.db.WithContext(ctx).Save(&uspts[i])
			continue
		}
		isAvailable = true
		break
	}
	if !isAvailable {
		us.ClearUserSubscriptionCache(ctx, uint(user.ID))

	}

	return isAvailable, nil
}

func (us *UserService) ActiveNewSubscriptionFromPending(ctx context.Context, userID int) (*models.UserSubscription, error) {
	// 检查是否已有激活的订阅
	activeUspts, err := us.GetUserSubscriptionsByUserIDAndStatus(ctx, uint(userID), "active")
	if err != nil {
		return nil, err
	}
	if len(activeUspts) > 0 {
		return nil, nil // 已有激活的订阅,不进行操作
	}

	uspts, err := us.GetUserSubscriptionsByUserIDAndStatus(ctx, uint(userID), "pending")
	if err != nil {
		return nil, err
	}
	if len(uspts) == 0 {
		return nil, errors.New("no pending subscription")
	}
	uspt := uspts[0]
	uspt.Status = "active"
	uspt.StartDate = time.Now()
	uspt.EndDate = uspt.StartDate.AddDate(0, 0, uspt.Subscription.Duration)
	err = us.db.WithContext(ctx).Save(&uspt).Error
	if err != nil {
		return nil, err
	}
	us.ClearUserSubscriptionCache(ctx, uint(userID))
	return &uspt, nil
}

func (us *UserService) IncrementTokenUsage(ctx context.Context, innerToken string) error {
	user, err := us.GetUserByInnerToken(ctx, innerToken)
	if err != nil {
		return err
	}
	uspts, err := us.GetUserSubscriptionsByUserIDAndStatus(ctx, uint(user.ID), "active")
	if err != nil {
		return err
	}
	if len(uspts) == 0 {
		return errors.New("no active subscription")
	}
	uspt := uspts[0]
	uspt.CursorAskUsage++
	if uspt.CursorAskUsage >= uspt.Subscription.CursorAskCount {
		uspt.Status = "expired"
		us.db.WithContext(ctx).Save(&uspt)
		us.ClearUserSubscriptionCache(ctx, uint(user.ID))
		us.ActiveNewSubscriptionFromPending(ctx, int(user.ID))
		return errors.New("cursor ask usage limit reached")
	}
	if uspt.EndDate.Before(time.Now()) {
		uspt.Status = "expired"
		us.db.WithContext(ctx).Save(&uspt)
		us.ClearUserSubscriptionCache(ctx, uint(user.ID))
		us.ActiveNewSubscriptionFromPending(ctx, int(user.ID))
		return errors.New("subscription expired")
	}
	err = us.db.WithContext(ctx).Save(&uspt).Error
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
