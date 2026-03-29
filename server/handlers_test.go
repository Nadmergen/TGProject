package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleCallStartResponse - checks handleCallStart response
func TestHandleCallStartResponse(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/calls/start", nil)
	w := httptest.NewRecorder()

	handleCallStart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleCallStart: expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("handleCallStart: expected Content-Type application/json, got %s", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("handleCallStart: response body is empty")
	}

	if body != `{"status":"call_started"}` {
		t.Errorf("handleCallStart: expected call_started, got %s", body)
	}
}

// TestHandleCallEndResponse - checks handleCallEnd response
func TestHandleCallEndResponse(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/calls/end", nil)
	w := httptest.NewRecorder()

	handleCallEnd(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleCallEnd: expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != `{"status":"call_ended"}` {
		t.Errorf("handleCallEnd: expected call_ended, got %s", body)
	}
}