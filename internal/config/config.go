package config

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env   string
	Port  int
	DBURL string

	AdminEmail          string
	AdminPassword       string
	AdminName           string
	AdminRole           string
	JWTSecret           string
	JWTAccessTTLMinutes int
	JWTRefreshTTLDays   int
	RedisAddr           string
	RedisPassword       string
	RedisDB             int
}

const (
	defaultJWTSecret     = "dev-secret-change-me"
	defaultAdminPassword = "changeme"
	defaultDBPassword    = "eventhub"
)

func Load() Config {
	env := getEnv("APP_ENV", "dev")
	port := getEnvInt("PORT", 8080)
	dbURL := buildDBURL()

	// admin config set up

	adminEmail := getEnv("ADMIN_EMAIL", "")
	adminPassword := getEnv("ADMIN_PASSWORD", "")
	adminName := getEnv("ADMIN_NAME", "Eventhub Admin")
	adminRole := getEnv("ADMIN_ROLE", "admin")
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-change-me")
	jwtTTL := getEnvInt("JWT_ACCESS_TTL_MINUTES", 60)
	jwtRefreshTTLDays := getEnvInt("JWT_REFRESH_TTL_DAYS", 14)
	redisAddr := getEnv("REDIS_ADDR", "127.0.0.1:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := getEnvInt("REDIS_DB", 0)

	return Config{
		Env:                 env,
		Port:                port,
		DBURL:               dbURL,
		AdminEmail:          adminEmail,
		AdminPassword:       adminPassword,
		AdminName:           adminName,
		AdminRole:           adminRole,
		JWTSecret:           jwtSecret,
		JWTAccessTTLMinutes: jwtTTL,
		JWTRefreshTTLDays:   jwtRefreshTTLDays,
		RedisAddr:           redisAddr,
		RedisPassword:       redisPassword,
		RedisDB:             redisDB,
	}
}

func ValidateForAPI(cfg Config) error {
	return validate(cfg, true, true)
}

func ValidateForWorker(cfg Config) error {
	return validate(cfg, false, false)
}

func validate(cfg Config, requireAuthConfig bool, requireRedis bool) error {
	var issues []string

	if cfg.Port <= 0 || cfg.Port > 65535 {
		issues = append(issues, "PORT must be between 1 and 65535")
	}

	dbURL, err := url.Parse(cfg.DBURL)
	if err != nil || dbURL == nil {
		issues = append(issues, "DB configuration is invalid (cannot parse DB URL)")
	} else {
		if dbURL.Scheme == "" || dbURL.Host == "" {
			issues = append(issues, "DB configuration is invalid (missing scheme or host)")
		}

		user := dbURL.User
		if user == nil || user.Username() == "" {
			issues = append(issues, "DB configuration is invalid (missing DB user)")
		}

		if user != nil {
			if pass, ok := user.Password(); !ok || strings.TrimSpace(pass) == "" {
				issues = append(issues, "DB configuration is invalid (missing DB password)")
			}
		}
	}

	if requireRedis && strings.TrimSpace(cfg.RedisAddr) == "" {
		issues = append(issues, "REDIS_ADDR is required")
	}

	if requireAuthConfig {
		if strings.TrimSpace(cfg.JWTSecret) == "" {
			issues = append(issues, "JWT_SECRET is required")
		}
		if strings.TrimSpace(cfg.AdminEmail) == "" {
			issues = append(issues, "ADMIN_EMAIL is required")
		}
		if strings.TrimSpace(cfg.AdminPassword) == "" {
			issues = append(issues, "ADMIN_PASSWORD is required")
		}
	}

	if isReleaseEnv(cfg.Env) {
		if requireAuthConfig {
			if cfg.JWTSecret == defaultJWTSecret {
				issues = append(issues, "JWT_SECRET must be changed from development default in release environments")
			}
			if len(cfg.JWTSecret) < 32 {
				issues = append(issues, "JWT_SECRET should be at least 32 characters in release environments")
			}
			if cfg.AdminPassword == defaultAdminPassword {
				issues = append(issues, "ADMIN_PASSWORD must be changed from default in release environments")
			}
		}

		if strings.Contains(cfg.DBURL, ":"+defaultDBPassword+"@") {
			issues = append(issues, "DB_PASSWORD must be changed from default in release environments")
		}
		if strings.Contains(strings.ToLower(cfg.DBURL), "sslmode=disable") {
			issues = append(issues, "DB_SSLMODE=disable is not allowed in release environments")
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(issues, "; "))
	}

	return nil
}

func isReleaseEnv(env string) bool {
	e := strings.ToLower(strings.TrimSpace(env))
	return e != "" && e != "dev" && e != "test"
}

func buildDBURL() string {
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "5433")
	user := getEnv("DB_USER", "eventhub")
	pass := getEnv("DB_PASSWORD", "eventhub")
	name := getEnv("DB_NAME", "eventhub")
	ssl := getEnv("DB_SSLMODE", "disable")

	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + name + "?sslmode=" + ssl
}

func WithTimeout(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		num, err := strconv.Atoi(v)

		if err != nil {
			fmt.Println(err)
		}

		return num
	}
	return fallback
}
