package goevent

import (
	"crypto/tls"
	"time"
)

// DriverType specifies which driver implementation to use
type DriverType string

const (
	// DriverMemory uses in-memory EventBus for same-process communication
	DriverMemory DriverType = "memory"
	// DriverRedis uses Redis pub/sub for distributed communication
	DriverRedis DriverType = "redis"
)

// Config configures the GoEvent instance
type Config struct {
	// Driver specifies which driver to use (memory or redis)
	Driver DriverType
	// Redis configuration (required when Driver is DriverRedis)
	Redis *RedisConfig
}

// RedisConfig configures the Redis driver
type RedisConfig struct {
	// Connection settings
	Addr     string // Redis server address (e.g., "localhost:6379")
	Password string // Redis password (empty string if no password)
	DB       int    // Redis database number (0-15)

	// Pub/Sub settings
	ChannelPrefix string // Prefix for Redis channels (default: "goevent:")

	// Serialization settings
	MaxEventSize int // Maximum event size in bytes (default: 1MB)

	// Connection pooling (optional)
	PoolSize     int // Maximum number of socket connections (default: 10)
	MinIdleConns int // Minimum number of idle connections (default: 0)

	// Timeouts (optional)
	DialTimeout  time.Duration // Timeout for establishing connections (default: 5s)
	ReadTimeout  time.Duration // Timeout for socket reads (default: 3s)
	WriteTimeout time.Duration // Timeout for socket writes (default: 3s)

	// TLS configuration (optional)
	TLSConfig *tls.Config
}

// setDefaults sets default values for optional RedisConfig fields
func (rc *RedisConfig) setDefaults() {
	if rc.ChannelPrefix == "" {
		rc.ChannelPrefix = "goevent:"
	}
	if rc.MaxEventSize == 0 {
		rc.MaxEventSize = 1024 * 1024 // 1MB
	}
	if rc.PoolSize == 0 {
		rc.PoolSize = 10
	}
	if rc.DialTimeout == 0 {
		rc.DialTimeout = 5 * time.Second
	}
	if rc.ReadTimeout == 0 {
		rc.ReadTimeout = 3 * time.Second
	}
	if rc.WriteTimeout == 0 {
		rc.WriteTimeout = 3 * time.Second
	}
}
