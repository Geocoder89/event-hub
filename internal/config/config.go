package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
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
	}
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
