package test

import (
	"context"
	"nursor-envoy-rpc/models"
	"nursor-envoy-rpc/service"
	"testing"
)

func TestGetAvailableTokenIdFromDB(t *testing.T) {
	service.KeepTokenQueueAvailable(context.Background())
}

func TestDispatchTokenForUser(t *testing.T) {
	service.GetDispatchInstance().DispatchTokenForUser(context.Background(), &models.User{ID: 25})
}

func TestIncrTokenUsage(t *testing.T) {
	service.GetDispatchInstance().IncrTokenUsage(context.Background(), "25")
}

func TestHandleTokenExpired(t *testing.T) {
	service.GetDispatchInstance().HandleTokenExpired(context.Background(), "1740")
}
