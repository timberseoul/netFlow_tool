package web

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticHandlerServesIndexWithoutRedirect(t *testing.T) {
	handler := mustNewStaticHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for root path, got %d", rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("expected no redirect for root path, got Location=%q", location)
	}

	if !strings.Contains(rec.Body.String(), "<title>netFlow_tool WebUI</title>") {
		t.Fatalf("expected index.html body, got %q", rec.Body.String())
	}
}

func TestStaticHandlerServesAssets(t *testing.T) {
	handler := mustNewStaticHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for asset path, got %d", rec.Code)
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if string(body) != "console.log('ok');" {
		t.Fatalf("unexpected asset body: %q", string(body))
	}
}

func TestStaticHandlerServesIndexForSpaRoute(t *testing.T) {
	handler := mustNewStaticHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/flows", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for SPA route, got %d", rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("expected no redirect for SPA route, got Location=%q", location)
	}
}

func mustNewStaticHandler(t *testing.T) http.Handler {
	t.Helper()

	distFS := fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<!doctype html><title>netFlow_tool WebUI</title>")},
		"assets/app.js":  &fstest.MapFile{Data: []byte("console.log('ok');")},
		"assets/app.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	handler, err := newStaticHandler(fs.FS(distFS))
	if err != nil {
		t.Fatalf("newStaticHandler: %v", err)
	}

	return handler
}
