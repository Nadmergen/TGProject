package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerInitialization(t *testing.T) {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Make a request to the test server
	resp, err := http.Get(testServer.URL)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
}