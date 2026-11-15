package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUsers(t *testing.T) {
	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()

	GetUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got []User
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(got) < 2 {
		t.Errorf("len(users) = %d, want >= 2", len(got))
	}
}

func TestCreateUser(t *testing.T) {
	body := bytes.NewBufferString(`{"name":"Charlie","email":"charlie@example.com"}`)
	req := httptest.NewRequest("POST", "/users", body)
	w := httptest.NewRecorder()

	CreateUser(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	var got User
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if got.Name != "Charlie" {
		t.Errorf("name = %q, want %q", got.Name, "Charlie")
	}
}
