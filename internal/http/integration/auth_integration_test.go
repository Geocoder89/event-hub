package integration__test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/geocoder89/eventhub/internal/config"
	apphttp "github.com/geocoder89/eventhub/internal/http"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testConfigAuth() config.Config {
	return config.Config{
		Env:                 "test",
		Port:                0,
		DBURL:               "",
		AdminEmail:          "admin@example.com",
		AdminPassword:       "ignored-in-tests",
		AdminName:           "Test Admin",
		AdminRole:           "admin",
		JWTSecret:           "test-secret-key",
		JWTAccessTTLMinutes: 60,
		JWTRefreshTTLDays:   7,
	}
}

func setupAuthTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := os.Getenv("TEST_DB_DSN")

	if dsn == "" {
		dsn = "postgres://eventhub:eventhub@127.0.0.1:5433/eventhub?sslmode=disable"
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)

	if err != nil {
		t.Fatalf("Failed to create pgx pool: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cfg := testConfigAuth()

	router := apphttp.NewRouter(logger, pool, cfg)

	return router, pool
}

func resetAuthDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	_, err := pool.Exec(context.Background(), `
		TRUNCATE refresh_tokens, users
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

// helpers

// type tokenResponse struct {
// 	AccessToken string `json:"accessToken"`
// }

func extraRefreshCookie(t *testing.T, response *http.Response) *http.Cookie {
	t.Helper()

	for _, c := range response.Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}

	t.Fatalf("refresh_token cookie not found in response")

	return nil
}

// function that runs a request and returns a recorder and parsed response for cookies

func doRequest(router http.Handler, method, path string, body string, cookies ...*http.Cookie) (*httptest.ResponseRecorder, *http.Response) {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))

	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w, w.Result()
}

func mustReadJSON[T any](t *testing.T, w *httptest.ResponseRecorder, out *T) {
	t.Helper()
	err := json.Unmarshal(w.Body.Bytes(), out)
	if err != nil {
		t.Fatalf("failed to unmarshal json: %v, body=%s", err, w.Body.String())
	}
}

func TestAuthIntegration_Signup_Login_Refresh_Logout(t *testing.T) {
	router, pool := setupAuthTestRouter(t)
	resetAuthDB(t, pool)

	defer resetAuthDB(t, pool)

	// sign up

	signupBody := `{"email":"sam@example.com","password":"password123","name":"Sam Doe"}`

	w, response := doRequest(router, http.MethodPost, "/signup", signupBody)

	if w.Code != http.StatusCreated {
		t.Fatalf("signup got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	var signupToken tokenResponse

	mustReadJSON(t, w, &signupToken)

	if strings.TrimSpace(signupToken.AccessToken) == "" {
		t.Fatalf("signup expected accessToken, got empty")
	}

	signupRefresh := extraRefreshCookie(t, response)

	// REFRESH (happy path)

	w2, response2 := doRequest(router, http.MethodPost, "/auth/refresh", "", signupRefresh)

	if w2.Code != http.StatusOK {
		t.Fatalf("refresh got status %d,want %d, body=%s", w2.Code, http.StatusOK, w2.Body.String())

	}
	var refreshTokenOk tokenResponse
	mustReadJSON(t, w2, &refreshTokenOk)

	if strings.TrimSpace(refreshTokenOk.AccessToken) == "" {
		t.Fatalf("refresh expected access token, got empty")
	}

	rotatedRefresh := extraRefreshCookie(t, response2)

	// Refresh with OLD Cookie should now fail (rotation)
	w3, _ := doRequest(router, http.MethodPost, "/auth/refresh", "", signupRefresh)
	if w3.Code != http.StatusUnauthorized {
		t.Fatalf("refresh(old cookie) got status %d, want %d, body=%s", w3.Code, http.StatusUnauthorized, w3.Body.String())
	}

	// Refreshing with new cookie should now succeed

	w4, _ := doRequest(router, http.MethodPost, "/auth/refresh", "", rotatedRefresh)

	if w4.Code != http.StatusOK {
		t.Fatalf("refresh(new cookie) got status %d, want %d, body=%s", w4.Code, http.StatusOK, w4.Body.String())
	}

	// LOGOUT should revoke and clear existing cookie

	w5, response5 := doRequest(router, http.MethodPost, "/auth/logout", "", rotatedRefresh)

	if w5.Code != http.StatusNoContent {
		t.Fatalf("logout got status %d, want %d, body=%s", w5.Code, http.StatusNoContent, w5.Body.String())
	}

	cleared := false

	for _, c := range response5.Cookies() {
		if c.Name == "refresh_token" && (c.MaxAge < 0 || c.Value == "") {
			cleared = true
		}
	}

	if !cleared {
		t.Fatalf("expected logout to clear refresh_token cookie")
	}

	// 6) REFRESH after logout should fail (best-effort revoke)
	w6, _ := doRequest(router, http.MethodPost, "/auth/refresh", "", rotatedRefresh)
	if w6.Code != http.StatusUnauthorized {
		t.Fatalf("refresh(after logout) got status %d, want %d, body=%s", w6.Code, http.StatusUnauthorized, w6.Body.String())
	}

}

func TestAuthIntegration_Refresh_MissingCookie(t *testing.T) {
	router, pool := setupAuthTestRouter(t)
	resetAuthDB(t, pool)
	defer resetAuthDB(t, pool)

	w, _ := doRequest(router, http.MethodPost, "/auth/refresh", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("refresh(missing cookie) got status %d, want %d, body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}

	var e apiErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Error.Code != "no_refresh" {
		t.Fatalf("expected no_refresh, got %s", e.Error.Code)
	}
}

func TestAuthIntegration_Login_InvalidCredentials(t *testing.T) {
	router, pool := setupAuthTestRouter(t)
	resetAuthDB(t, pool)
	defer resetAuthDB(t, pool)

	// no user created
	body := `{"email":"nope@example.com","password":"wrong"}`
	w, _ := doRequest(router, http.MethodPost, "/login", body)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("login(invalid creds) got status %d, want %d, body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
}
