package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	handleHealthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("unexpected body: %s", body)
	}
	if !strings.Contains(body, version) {
		t.Fatalf("body missing version %s: %s", version, body)
	}
}

func TestIndexServesBrandedPage(t *testing.T) {
	rec := httptest.NewRecorder()
	handleIndex(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hush") {
		t.Fatal("index page missing brand name")
	}
}

func TestIndexUnknownPathIs404(t *testing.T) {
	rec := httptest.NewRecorder()
	handleIndex(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
