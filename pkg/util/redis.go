package util

import (
	"os"

	"github.com/redis/go-redis/v9"
)

// Config holds the application configuration
type RedisConfig struct {
	RedisURL      string
	RedisUsername string
	RedisPassword string
}

// getEnv gets the value of the environment variable key or returns defaultVal if the variable is not set.
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// LoadConfig loads configuration from environment variables
func LoadRedisConfig() (RedisConfig, error) {
	return RedisConfig{
		RedisURL:      getEnv("REDIS_ADDR", "localhost:6379"),
		RedisUsername: getEnv("REDIS_USERNAME", ""),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
	}, nil
}

func NewRedisConfig() *RedisConfig {
	cfg, err := LoadRedisConfig()
	if err != nil {
		panic(err)
	}

	return &cfg
}

func NewRedisClient() *redis.Client {
	redisconfig := NewRedisConfig()
	redisclient := redis.NewClient(&redis.Options{
		Addr:     redisconfig.RedisURL,
		Password: redisconfig.RedisPassword,
		Username: redisconfig.RedisUsername,
		DB:       0,
	})

	return redisclient
}
