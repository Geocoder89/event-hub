package config

import "testing"

func baseConfig(env string) Config {
	return Config{
		Env:                 env,
		Port:                8080,
		DBURL:               "postgres://eventhub:strong-db-password@db:5432/eventhub?sslmode=require",
		AdminEmail:          "admin@example.com",
		AdminPassword:       "very-strong-admin-password",
		JWTSecret:           "this-is-a-very-strong-jwt-secret-value",
		JWTAccessTTLMinutes: 60,
		JWTRefreshTTLDays:   14,
		RedisAddr:           "redis:6379",
	}
}

func TestValidateForAPI_AllowsDevDefaults(t *testing.T) {
	cfg := baseConfig("dev")
	cfg.JWTSecret = defaultJWTSecret
	cfg.AdminPassword = defaultAdminPassword
	cfg.DBURL = "postgres://eventhub:eventhub@127.0.0.1:5433/eventhub?sslmode=disable"

	if err := ValidateForAPI(cfg); err != nil {
		t.Fatalf("ValidateForAPI(dev) returned error: %v", err)
	}
}

func TestValidateForAPI_RejectsReleaseInsecureDefaults(t *testing.T) {
	cfg := baseConfig("prod")
	cfg.JWTSecret = defaultJWTSecret
	cfg.AdminPassword = defaultAdminPassword
	cfg.DBURL = "postgres://eventhub:eventhub@db:5432/eventhub?sslmode=disable"

	err := ValidateForAPI(cfg)
	if err == nil {
		t.Fatal("expected release validation error, got nil")
	}
}

func TestValidateForAPI_RejectsMissingRedis(t *testing.T) {
	cfg := baseConfig("dev")
	cfg.RedisAddr = ""

	err := ValidateForAPI(cfg)
	if err == nil {
		t.Fatal("expected redis validation error, got nil")
	}
}

func TestValidateForWorker_DoesNotRequireAdminOrRedis(t *testing.T) {
	cfg := baseConfig("prod")
	cfg.AdminEmail = ""
	cfg.AdminPassword = ""
	cfg.RedisAddr = ""

	if err := ValidateForWorker(cfg); err != nil {
		t.Fatalf("ValidateForWorker(prod) returned error: %v", err)
	}
}

func TestValidateForWorker_RejectsReleaseInsecureDB(t *testing.T) {
	cfg := baseConfig("prod")
	cfg.AdminEmail = ""
	cfg.AdminPassword = ""
	cfg.RedisAddr = ""
	cfg.DBURL = "postgres://eventhub:eventhub@db:5432/eventhub?sslmode=disable"

	err := ValidateForWorker(cfg)
	if err == nil {
		t.Fatal("expected worker release validation error, got nil")
	}
}
