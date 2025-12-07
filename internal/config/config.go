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
}

func Load() Config {
	env := getEnv("APP_ENV", "dev")
	port := getEnvInt("PORT",8080)

	return Config{
		Env: env,
		Port: port,
	}
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