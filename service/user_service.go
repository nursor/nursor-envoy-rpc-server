package service

import (
	"context"
	"errors"
	"nursor-envoy-rpc/helper"
	"nursor-envoy-rpc/models"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"gorm.io/gorm"
)

// UserService manages user-related operations with Redis caching and token validation.
type UserService struct {
	defaultRedis      *redis.Client
	db                *gorm.DB
	conn              sqlx.SqlConn
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
	err := us.db.WithContext(ctx).Where("inner_token = ?", innerToken).First(&user).Error
	if err != nil {
		return nil, err
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

	uspt := models.UserSubscription{}
	uspts, err := uspt.FindUserSubscriptionsByUserIDAndStatus(us.db, uint(user.ID), "active")
	if err != nil {
		return false, err
	}
	if len(uspts) == 0 {
		return false, nil
	}

	isAvailable := false
	for _, uspt = range uspts {
		if uspt.EndDate.Before(time.Now()) {
			uspt.Status = "expired"
			us.db.WithContext(ctx).Save(&uspt)
			continue
		}
		if uspt.UsedTraffic >= *uspt.Subscription.TrafficLimit {
			uspt.Status = "expired"
			us.db.WithContext(ctx).Save(&uspt)
			continue
		}
		if uspt.CursorAskUsage >= uspt.Subscription.CursorAskCount {
			uspt.Status = "expired"
			us.db.WithContext(ctx).Save(&uspt)
			continue
		}
		isAvailable = true
		break
	}

	return isAvailable, nil
}

func (us *UserService) ActiveNewSubscriptionFromPending(ctx context.Context, userID int) (*models.UserSubscription, error) {
	uspt := models.UserSubscription{}
	// 检查是否已有激活的订阅
	activeUspts, err := uspt.FindUserSubscriptionsByUserIDAndStatus(us.db, uint(userID), "active")
	if err != nil {
		return nil, err
	}
	if len(activeUspts) > 0 {
		return nil, nil // 已有激活的订阅,不进行操作
	}

	// // 使用分布式锁确保并发安全
	// lockKey := fmt.Sprintf("user_subscription_lock:%d", userID)
	// lock := redislock.New(&redislock.GoRedisClient{Client: us.defaultRedis})
	// lockCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	// defer cancel()

	// locker, err := lock.Obtain(lockCtx, lockKey, 10*time.Second, nil)
	// if err != nil {
	// 	return nil, fmt.Errorf("获取锁失败: %v", err)
	// }
	// defer locker.Release(ctx)

	uspts, err := uspt.FindUserSubscriptionsByUserIDAndStatus(us.db, uint(userID), "pending")
	if err != nil {
		return nil, err
	}
	if len(uspts) == 0 {
		return nil, errors.New("no pending subscription")
	}
	uspt = uspts[0]
	uspt.Status = "active"
	uspt.StartDate = time.Now()
	uspt.EndDate = uspt.StartDate.AddDate(0, 0, uspt.Subscription.Duration)
	err = us.db.WithContext(ctx).Save(&uspt).Error
	if err != nil {
		return nil, err
	}
	return &uspt, nil
}

func (us *UserService) IncrementTokenUsage(ctx context.Context, innerToken string) error {
	user, err := us.GetUserByInnerToken(ctx, innerToken)
	if err != nil {
		return err
	}
	uspt := models.UserSubscription{}
	uspts, err := uspt.FindUserSubscriptionsByUserIDAndStatus(us.db, uint(user.ID), "active")
	if err != nil {
		return err
	}
	if len(uspts) == 0 {
		return errors.New("no active subscription")
	}
	uspt = uspts[0]
	uspt.CursorAskUsage++
	if uspt.CursorAskUsage >= uspt.Subscription.CursorAskCount {
		uspt.Status = "expired"
		us.db.WithContext(ctx).Save(&uspt)
		us.ActiveNewSubscriptionFromPending(ctx, int(user.ID))
		return errors.New("cursor ask usage limit reached")
	}
	if uspt.EndDate.Before(time.Now()) {
		uspt.Status = "expired"
		us.db.WithContext(ctx).Save(&uspt)
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
