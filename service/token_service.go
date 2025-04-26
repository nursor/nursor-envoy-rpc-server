package service

import (
	"context"
	"fmt"
	"nursor-envoy-rpc/helper"
	"nursor-envoy-rpc/models"
	"sync"

	"gorm.io/gorm"
)

var tokenServiceOnce sync.Once
var tokenServiceInstance *TokenService
var TokenMaxUsage = 50

type TokenService struct {
	// TokenID is the unique identifier for the token.
	redisOperator   *helper.RedisOperator
	tokenPersistent *helper.TokenPersistent
	db              *gorm.DB
}

func GetTokenServiceInstance() *TokenService {
	tokenServiceOnce.Do(func() {
		tokenServiceInstance = &TokenService{
			redisOperator:   helper.GetInstanceRedisOperator(),
			tokenPersistent: helper.GetTPInstance(),
			db:              helper.GetNewDB(),
		}
	})
	return tokenServiceInstance
}

func (ts *TokenService) GetTokenByUserId(ctx context.Context, userID int) (string, error) {

	tokenID, err := ts.redisOperator.GetTokenID(ctx, fmt.Sprint(userID))
	if err != nil && tokenID == "" {
		tokenID, err = ts.redisOperator.AssignNewToken(ctx)
		if err != nil {
			return "", err
		}
		if tokenID == "" {
			return "", fmt.Errorf("no available token found")
		}
		err = ts.redisOperator.SetToken(ctx, fmt.Sprint(userID), tokenID)
		if err != nil {
			return "", err
		}
	}
	return tokenID, nil
}

func (ts *TokenService) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	var user models.User
	err := ts.db.WithContext(ctx).Where("inner_token = ?", tokenID).First(&user).Error
	if err != nil {
		return err
	}
	_, err = ts.redisOperator.IncrementTokenUsage(ctx, fmt.Sprint(user.ID), 1)

	return err
}

func (ts *TokenService) CheckTokenUsage(ctx context.Context, tokenID string) (bool, error) {
	usage, err := ts.redisOperator.GetTokenUsage(ctx, tokenID)
	if err != nil {
		return false, err
	}
	if usage >= int64(TokenMaxUsage) {

	}
	return true, nil
}

func (ts *TokenService) HandleUserLeave(ctx context.Context, userId string) error {
	_, err := ts.redisOperator.DeleteToken(ctx, userId)
	return err
}
