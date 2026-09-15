package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func spaFS() fs.FS {
	return fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html><title>sdtviewer spa</title>")},
		"assets/app.js":   {Data: []byte("console.log(1)")},
		"assets/app.css":  {Data: []byte("body{}")},
		"favicon.svg":     {Data: []byte("<svg/>")},
		"nested/page.txt": {Data: []byte("x")},
	}
}

func TestSpaFileHandlerRoot(t *testing.T) {
	h := spaFileHandler(spaFS())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := rec.Body.String(); body == "" || !strings.Contains(body, "sdtviewer spa") {
		t.Errorf("root body = %q", body)
	}
}

func TestSpaFileHandlerAsset(t *testing.T) {
	h := spaFileHandler(spaFS())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "console.log") {
		t.Errorf("asset status = %d body = %q", rec.Code, rec.Body.String())
	}
}

func TestSpaFileHandlerHashRouteFallback(t *testing.T) {
	h := spaFileHandler(spaFS())
	for _, path := range []string{"/wiki/graph", "/deep/link/here"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "sdtviewer spa") {
			t.Errorf("%s: status = %d body = %q", path, rec.Code, rec.Body.String())
		}
	}
}

func TestSpaFileHandlerMissingAsset(t *testing.T) {
	h := spaFileHandler(spaFS())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for missing asset", rec.Code)
	}
}

func TestSpaHandlerWithoutBuild(t *testing.T) {
	// viewer/dist only carries .gitkeep in a clean checkout: no index.html.
	if _, ok := spaHandler(); ok {
		t.Skip("embedded SPA build present; skipping empty-asset assertion")
	}
}
