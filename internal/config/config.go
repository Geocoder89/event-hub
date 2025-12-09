package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)


type Config struct {
	Env string
	Port int
	DBURL string
}

func Load() Config {
	env := getEnv("APP_ENV", "dev")
	port := getEnvInt("PORT",8080)
	dbURL := buildDBURL()

	return Config{
		Env: env,
		Port: port,
		DBURL: dbURL,
	}
}

func buildDBURL() string {
	host := getEnv("DB_HOST","127.0.0.1")
	port := getEnv("DB_PORT","5432")
	user := getEnv("DB_USER","eventhub")
	pass := getEnv("DB_PASSWORD","eventhub")
	name := getEnv("DB_NAME", "eventhub")
	ssl := getEnv("DB_SSLMODE", "disable")


	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + name + "?sslmode=" + ssl
}

func WithTimeout(duration time.Duration)(context.Context, context.CancelFunc){
	return context.WithTimeout(context.Background(),duration)
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