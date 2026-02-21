package integration__test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type registrationResponse struct {
	ID           string     `json:"id"`
	EventID      string     `json:"eventId"`
	UserID       string     `json:"userId"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	CheckInToken string     `json:"checkInToken"`
	CheckedInAt  *time.Time `json:"checkedInAt"`
}

func doAuthedJSONRequest(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func loginAndGetToken(t *testing.T, router http.Handler, email, password string) string {
	t.Helper()

	body := `{"email":"` + email + `","password":"` + password + `"}`
	w, _ := doRequest(router, http.MethodPost, "/login", body)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: status=%d body=%s", w.Code, w.Body.String())
	}

	var resp tokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}

	if resp.AccessToken == "" {
		t.Fatalf("missing access token on login response")
	}
	return resp.AccessToken
}

func createAdminAuthToken(t *testing.T, router *gin.Engine, pool *pgxpool.Pool, email string) string {
	t.Helper()

	_ = signupAndGetToken(t, router, email)
	_, err := pool.Exec(context.Background(), `UPDATE users SET role = 'admin' WHERE email = $1`, email)
	if err != nil {
		t.Fatalf("failed to promote admin user: %v", err)
	}
	return loginAndGetToken(t, router, email, "StrongPassword123!")
}

func TestRegistrationCheckInIntegration_Success(t *testing.T) {
	router, pool := setupTestRouter(t)
	resetDB(t, pool)
	defer resetDB(t, pool)

	eventID := seedEvent(t, pool, 2)

	userToken := signupAndGetToken(t, router, "attendee@example.com")
	regBody := `{"name":"Attendee","email":"attendee@example.com"}`
	regW := doAuthedJSONRequest(router, http.MethodPost, "/events/"+eventID+"/register", regBody, userToken)
	if regW.Code != http.StatusCreated {
		t.Fatalf("register failed: status=%d body=%s", regW.Code, regW.Body.String())
	}

	var reg registrationResponse
	if err := json.Unmarshal(regW.Body.Bytes(), &reg); err != nil {
		t.Fatalf("failed to decode registration response: %v", err)
	}
	if reg.CheckInToken == "" {
		t.Fatalf("expected checkInToken in registration response")
	}

	adminToken := createAdminAuthToken(t, router, pool, "admin-checkin@example.com")

	checkInBody := `{"token":"` + reg.CheckInToken + `"}`
	checkInW := doAuthedJSONRequest(router, http.MethodPost, "/admin/events/"+eventID+"/registrations/check-in", checkInBody, adminToken)
	if checkInW.Code != http.StatusOK {
		t.Fatalf("check-in failed: status=%d body=%s", checkInW.Code, checkInW.Body.String())
	}

	var checked registrationResponse
	if err := json.Unmarshal(checkInW.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to decode check-in response: %v", err)
	}
	if checked.CheckedInAt == nil {
		t.Fatalf("expected checkedInAt to be set after check-in")
	}

	var count int
	err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM registrations WHERE id = $1 AND checked_in_at IS NOT NULL`, reg.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to verify checked-in row: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one checked-in row, got %d", count)
	}
}

func TestRegistrationCheckInIntegration_AlreadyCheckedIn(t *testing.T) {
	router, pool := setupTestRouter(t)
	resetDB(t, pool)
	defer resetDB(t, pool)

	eventID := seedEvent(t, pool, 2)

	userToken := signupAndGetToken(t, router, "attendee2@example.com")
	regW := doAuthedJSONRequest(router, http.MethodPost, "/events/"+eventID+"/register", `{"name":"Attendee","email":"attendee2@example.com"}`, userToken)
	if regW.Code != http.StatusCreated {
		t.Fatalf("register failed: status=%d body=%s", regW.Code, regW.Body.String())
	}
	var reg registrationResponse
	if err := json.Unmarshal(regW.Body.Bytes(), &reg); err != nil {
		t.Fatalf("failed to decode registration response: %v", err)
	}

	adminToken := createAdminAuthToken(t, router, pool, "admin-checkin2@example.com")

	body := `{"token":"` + reg.CheckInToken + `"}`
	w1 := doAuthedJSONRequest(router, http.MethodPost, "/admin/events/"+eventID+"/registrations/check-in", body, adminToken)
	if w1.Code != http.StatusOK {
		t.Fatalf("first check-in failed: status=%d body=%s", w1.Code, w1.Body.String())
	}

	w2 := doAuthedJSONRequest(router, http.MethodPost, "/admin/events/"+eventID+"/registrations/check-in", body, adminToken)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second check-in got status=%d want=%d body=%s", w2.Code, http.StatusConflict, w2.Body.String())
	}

	var e apiErrorResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &e); err != nil {
		t.Fatalf("failed to decode conflict response: %v", err)
	}
	if e.Error.Code != "already_checked_in" {
		t.Fatalf("expected already_checked_in, got %s", e.Error.Code)
	}
}

func TestRegistrationCheckInIntegration_InvalidToken(t *testing.T) {
	router, pool := setupTestRouter(t)
	resetDB(t, pool)
	defer resetDB(t, pool)

	eventID := seedEvent(t, pool, 2)

	adminToken := createAdminAuthToken(t, router, pool, "admin-checkin3@example.com")

	w := doAuthedJSONRequest(router, http.MethodPost, "/admin/events/"+eventID+"/registrations/check-in", `{"token":"not-a-real-token"}`, adminToken)
	if w.Code != http.StatusNotFound {
		t.Fatalf("invalid token got status=%d want=%d body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}
}
