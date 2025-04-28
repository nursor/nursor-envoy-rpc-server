package test

import (
	"context"
	"nursor-envoy-rpc/service"
	"testing"
)

func TestGetAvailableTokenIdFromDB(t *testing.T) {
	service.KeepTokenQueueAvailable(context.Background())
}

func TestDispatchTokenForUser(t *testing.T) {
	service.GetDispatchInstance().DispatchTokenForUser(context.Background(), "1")
}

func TestIncrTokenUsage(t *testing.T) {
	service.GetDispatchInstance().IncrTokenUsage(context.Background(), "25")
}
