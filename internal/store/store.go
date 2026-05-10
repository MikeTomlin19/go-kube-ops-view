package store

import (
	"fmt"
	"strings"
	"time"
)

// Config holds store configuration
type Config struct {
	Type     string        `yaml:"type" env:"STORE_TYPE" default:"memory"`
	Redis    RedisConfig   `yaml:"redis"`
	TokenTTL time.Duration `yaml:"token_ttl" env:"TOKEN_TTL" default:"24h"`
}

// NewStore creates a new store based on configuration
func NewStore(config Config) (Store, error) {
	switch strings.ToLower(config.Type) {
	case "memory", "":
		if config.TokenTTL > 0 {
			return NewMemoryStoreWithTTL(config.TokenTTL), nil
		}
		return NewMemoryStore(), nil

	case "redis":
		if config.Redis.Addr == "" {
			return nil, fmt.Errorf("Redis address is required for Redis store")
		}

		config.Redis.TokenTTL = config.TokenTTL
		return NewRedisStore(config.Redis)

	case "redis-cluster":
		if len(strings.Split(config.Redis.Addr, ",")) < 2 {
			return nil, fmt.Errorf("Redis cluster requires multiple addresses")
		}

		addrs := strings.Split(config.Redis.Addr, ",")
		for i, addr := range addrs {
			addrs[i] = strings.TrimSpace(addr)
		}

		return NewRedisClusterStore(addrs, config.Redis.Password, config.TokenTTL)

	default:
		return nil, fmt.Errorf("unsupported store type: %s", config.Type)
	}
}

// DefaultConfig returns a default store configuration
func DefaultConfig() Config {
	return Config{
		Type:     "memory",
		TokenTTL: 24 * time.Hour,
		Redis: RedisConfig{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
			TokenTTL: 24 * time.Hour,
		},
	}
}
