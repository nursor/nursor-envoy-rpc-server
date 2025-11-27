package helper

import (
	"os"
	"strconv"

	"github.com/go-redis/redis/v8"
)

// RedisOperator is a singleton for managing Redis operations related to tokens and users.
type RedisOperator struct {
	redis            *redis.Client
	appName          string
	userTokenPrefix  string
	tokenUsersPrefix string
	tokenUsagePrefix string
	userUsagePrefix  string
	tokenListKey     string
	initialized      bool
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
