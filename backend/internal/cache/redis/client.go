package redis

import (
	"context"
	"fmt"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

// Config defines bounded connection settings for the ephemeral Redis layer.
type Config struct {
	Address      string
	Password     string
	Database     int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Open creates a Redis client and verifies that the configured server is reachable.
func Open(ctx context.Context, cfg Config) (*redisv9.Client, error) {
	client := redisv9.NewClient(&redisv9.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.Database,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		MaxRetries:   2,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("verify Redis connectivity: %w", err)
	}

	return client, nil
}
