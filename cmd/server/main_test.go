package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRoutesAndReadiness(t *testing.T) {
	dataDir := t.TempDir()
	if err := ensureDataDirectories(dataDir); err != nil {
		t.Fatal(err)
	}
	app := &server{config: config{DataDir: dataDir}}
	handler := app.routes()
	for _, test := range []struct {
		path   string
		status int
	}{
		{"/healthz", 200}, {"/readyz", 200}, {"/api/v1/site", 200}, {"/api/v1/missing", 404},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
		if response.Code != test.status {
			t.Fatalf("%s: got %d, want %d", test.path, response.Code, test.status)
		}
	}
	if err := os.Remove(filepath.Join(dataDir, "media")); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/readyz", nil))
	if response.Code != 503 {
		t.Fatalf("missing data directory: got %d, want 503", response.Code)
	}
}
