package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ridenow/ridenow/internal/db"
)

func newTestRouter(t *testing.T) chi.Router {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := chi.NewRouter()
	New(database).Routes(r)
	return r
}

func TestRegisterHappyPath(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/register",
		strings.NewReader(`{"email":"u@example.com","password":"s3cret","role":"rider"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201; body: %s", rec.Code, rec.Body)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["id"] == "" || body["id"] == nil {
		t.Fatal("id should be non-empty")
	}
	if body["email"] != "u@example.com" {
		t.Fatalf("email: got %v", body["email"])
	}
	if body["role"] != "rider" {
		t.Fatalf("role: got %v", body["role"])
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	r := newTestRouter(t)

	do := func() int {
		req := httptest.NewRequest(http.MethodPost, "/register",
			strings.NewReader(`{"email":"dup@example.com","password":"pass","role":"rider"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do(); code != http.StatusCreated {
		t.Fatalf("first register: got %d, want 201", code)
	}
	if code := do(); code != http.StatusConflict {
		t.Fatalf("second register: got %d, want 409", code)
	}
}

func TestLoginCorrectCredentials(t *testing.T) {
	r := newTestRouter(t)

	reg := httptest.NewRequest(http.MethodPost, "/register",
		strings.NewReader(`{"email":"u@example.com","password":"s3cret","role":"rider"}`))
	reg.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	r.ServeHTTP(regRec, reg)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: got %d", regRec.Code)
	}

	var regBody map[string]any
	if err := json.NewDecoder(regRec.Body).Decode(&regBody); err != nil {
		t.Fatalf("decode register: %v", err)
	}

	login := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"email":"u@example.com","password":"s3cret"}`))
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: got %d, want 200; body: %s", loginRec.Code, loginRec.Body)
	}

	var loginBody map[string]any
	if err := json.NewDecoder(loginRec.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if loginBody["id"] != regBody["id"] {
		t.Fatalf("id mismatch: login %v != register %v", loginBody["id"], regBody["id"])
	}
	if loginBody["role"] != "rider" {
		t.Fatalf("role: got %v", loginBody["role"])
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r := newTestRouter(t)

	reg := httptest.NewRequest(http.MethodPost, "/register",
		strings.NewReader(`{"email":"u@example.com","password":"s3cret","role":"rider"}`))
	reg.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	r.ServeHTTP(regRec, reg)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: got %d", regRec.Code)
	}

	login := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"email":"u@example.com","password":"wrongpass"}`))
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d, want 401", loginRec.Code)
	}
}
