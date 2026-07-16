package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"server/routes"
)

// TestPing checks that the ping endpoint returns the expected JSON response.
func TestPing(t *testing.T) {
	router := routes.SetupRouter()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	expectedBody := `{"message":"pong"}`
	if response.Body.String() != expectedBody {
		t.Fatalf("expected body %s, got %s", expectedBody, response.Body.String())
	}
}
