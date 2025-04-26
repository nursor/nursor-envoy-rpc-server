package service

import (
	"context"
	"fmt"
	"nursor-envoy-rpc/helper"
	"sync"
)

var tokenServiceOnce sync.Once
var tokenServiceInstance *TokenService

type TokenService struct {
	// TokenID is the unique identifier for the token.
	redisOperator   *helper.RedisOperator
	tokenPersistent *helper.TokenPersistent
}

func NewTokenService(tokenID string) *TokenService {
	tokenServiceOnce.Do(func() {
		tokenServiceInstance = &TokenService{
			redisOperator:   helper.GetInstanceRedisOperator(),
			tokenPersistent: helper.GetTPInstance(),
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

func (ts *TokenService) HandleUserLeave(ctx context.Context, userId string) error {
	_, err := ts.redisOperator.DeleteToken(ctx, userId)
	return err
}
