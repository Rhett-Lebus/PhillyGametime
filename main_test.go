package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeServiceWorker(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	rec := httptest.NewRecorder()

	serveServiceWorker(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Service-Worker-Allowed"); got != "/" {
		t.Fatalf("Service-Worker-Allowed = %q, want %q", got, "/")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-cache")
	}
	if !strings.Contains(rec.Body.String(), "self.addEventListener('fetch'") {
		t.Fatal("service worker response does not contain fetch handler")
	}
}
