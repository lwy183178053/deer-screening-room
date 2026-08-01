package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestNormalizeEmailAndPassword(t *testing.T) {
	email, ok := normalizeEmail("User@Example.COM")
	if !ok || email != "user@example.com" {
		t.Fatalf("email=%q ok=%v", email, ok)
	}
	if _, ok := normalizeEmail("Name <user@example.com>"); ok {
		t.Fatal("accepted display address")
	}
	if validPassword("short") || !validPassword("12345678") {
		t.Fatal("password boundary failed")
	}
}

func TestRequestedSeed(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/videos?seed=42", nil)
	seed, ok := requestedSeed(response, request)
	if !ok || seed != 42 {
		t.Fatalf("seed=%d ok=%v", seed, ok)
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/api/v1/videos?seed=invalid", nil)
	if _, ok := requestedSeed(response, request); ok || response.Code != 422 {
		t.Fatalf("invalid seed accepted, status=%d", response.Code)
	}
}
