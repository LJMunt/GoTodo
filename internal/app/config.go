package app

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

type ServerConfig struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	MaxBodyBytes      int64
	AdminMaxBodyBytes int64
	TrustedProxies    []netip.Prefix
	ShutdownTimeout   time.Duration
}

type LoggingConfig struct {
	LevelRefreshInterval time.Duration
	ConfigWatchInterval  time.Duration
}

type Config struct {
	DatabaseURL    string
	MigrationsPath string
	Port           string
	Server         ServerConfig
	Logging        LoggingConfig
}

func DefaultConfig() Config {
	return Config{
		MigrationsPath: "internal/db/migrations",
		Port:           "8080",
		Server: ServerConfig{
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       20 * time.Second,
			WriteTimeout:      20 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 20, // 1 MiB
			MaxBodyBytes:      10 << 20,
			AdminMaxBodyBytes: 50 << 20,
			ShutdownTimeout:   10 * time.Second,
		},
		Logging: LoggingConfig{
			LevelRefreshInterval: 5 * time.Second,
			ConfigWatchInterval:  1 * time.Minute,
		},
	}
}

func LoadConfig() (Config, error) {
	cfg := DefaultConfig()

	cfg.MigrationsPath = envString("MIGRATIONS_PATH", cfg.MigrationsPath)
	cfg.Port = envString("PORT", cfg.Port)

	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	cfg.Server.ReadHeaderTimeout = envDuration("HTTP_READ_HEADER_TIMEOUT", cfg.Server.ReadHeaderTimeout)
	cfg.Server.ReadTimeout = envDuration("HTTP_READ_TIMEOUT", cfg.Server.ReadTimeout)
	cfg.Server.WriteTimeout = envDuration("HTTP_WRITE_TIMEOUT", cfg.Server.WriteTimeout)
	cfg.Server.IdleTimeout = envDuration("HTTP_IDLE_TIMEOUT", cfg.Server.IdleTimeout)
	cfg.Server.MaxHeaderBytes = envInt("HTTP_MAX_HEADER_BYTES", cfg.Server.MaxHeaderBytes)
	cfg.Server.MaxBodyBytes = envInt64("HTTP_MAX_BODY_BYTES", cfg.Server.MaxBodyBytes)
	cfg.Server.AdminMaxBodyBytes = envInt64("HTTP_ADMIN_MAX_BODY_BYTES", cfg.Server.AdminMaxBodyBytes)
	cfg.Server.ShutdownTimeout = envDuration("HTTP_SHUTDOWN_TIMEOUT", cfg.Server.ShutdownTimeout)

	cfg.Logging.LevelRefreshInterval = envDuration("LOG_LEVEL_REFRESH_INTERVAL", cfg.Logging.LevelRefreshInterval)
	cfg.Logging.ConfigWatchInterval = envDuration("CONFIG_WATCH_INTERVAL", cfg.Logging.ConfigWatchInterval)

	trusted, err := parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, err
	}
	cfg.Server.TrustedProxies = trusted

	return cfg, nil
}

func envString(key, def string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	return val
}

func envDuration(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		return def
	}
	return parsed
}

func envInt(key string, def int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func envInt64(key string, def int64) int64 {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func parseTrustedProxies(raw string) ([]netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	out := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("invalid TRUSTED_PROXIES prefix: %q", part)
			}
			out = append(out, p)
			continue
		}
		addr, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("invalid TRUSTED_PROXIES address: %q", part)
		}
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out, nil
}
