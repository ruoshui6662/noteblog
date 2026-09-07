package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"markdown-docs/auth"
	"markdown-docs/content"
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
	redirectResponse := httptest.NewRecorder()
	handler.ServeHTTP(redirectResponse, httptest.NewRequest("GET", "/api/v1/auth/setup", nil))
	if redirectResponse.Code != 307 || redirectResponse.Header().Get("Location") != "/admin" {
		t.Fatalf("setup page redirect = %d %q", redirectResponse.Code, redirectResponse.Header().Get("Location"))
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

func TestAuthAPI(t *testing.T) {
	dataDir := t.TempDir()
	if err := ensureDataDirectories(dataDir); err != nil {
		t.Fatal(err)
	}
	store, err := auth.Open(filepath.Join(dataDir, "noteblog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := &server{config: config{DataDir: dataDir}, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), auth: store}
	handler := app.routes()
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, httptest.NewRequest("POST", "/api/v1/auth/setup", strings.NewReader(`{"username":"admin","password":"correct horse battery staple"}`)))
	if setup.Code != 201 {
		t.Fatalf("setup status = %d: %s", setup.Code, setup.Body.String())
	}
	secondSetup := httptest.NewRecorder()
	handler.ServeHTTP(secondSetup, httptest.NewRequest("POST", "/api/v1/auth/setup", strings.NewReader(`{"username":"other","password":"correct horse battery staple"}`)))
	if secondSetup.Code != 409 {
		t.Fatalf("second setup status = %d", secondSetup.Code)
	}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"correct horse battery staple"}`)))
	if login.Code != 200 || login.Header().Get("Set-Cookie") == "" {
		t.Fatalf("login = %d %q", login.Code, login.Header().Get("Set-Cookie"))
	}
	cookie := login.Result().Cookies()[0]
	meRequest := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meRequest.AddCookie(cookie)
	me := httptest.NewRecorder()
	handler.ServeHTTP(me, meRequest)
	if me.Code != 200 || !strings.Contains(me.Body.String(), `"username":"admin"`) {
		t.Fatalf("me = %d %s", me.Code, me.Body.String())
	}
	logoutRequest := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	logout := httptest.NewRecorder()
	handler.ServeHTTP(logout, logoutRequest)
	if logout.Code != 200 {
		t.Fatalf("logout = %d %s", logout.Code, logout.Body.String())
	}
	meAfterLogout := httptest.NewRecorder()
	handler.ServeHTTP(meAfterLogout, meRequest)
	if meAfterLogout.Code != 401 {
		t.Fatalf("me after logout = %d", meAfterLogout.Code)
	}
}

func TestAdminDocumentAPIRequiresSessionAndUsesVersions(t *testing.T) {
	dataDir := t.TempDir()
	if err := ensureDataDirectories(dataDir); err != nil {
		t.Fatal(err)
	}
	store, err := auth.Open(filepath.Join(dataDir, "noteblog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := &server{
		config: config{DataDir: dataDir},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:  content.NewStore(filepath.Join(dataDir, "content")),
		auth:   store,
	}
	handler := app.routes()
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest("GET", "/api/v1/admin/docs", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized list = %d", unauthorized.Code)
	}
	if _, err := store.Setup("admin", "short"); err != nil {
		t.Fatal(err)
	}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"short"}`)))
	if login.Code != http.StatusOK {
		t.Fatalf("login = %d %s", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	request := httptest.NewRequest("POST", "/api/v1/admin/docs", strings.NewReader(`{"path":"managed.md","content":"# 第一版\n","metadata":{"title":"管理文档","description":"结构化说明","tags":["指南","Docker"],"draft":true,"order":2}}`))
	request.AddCookie(cookie)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, request)
	if created.Code != http.StatusCreated || created.Header().Get("ETag") == "" {
		t.Fatalf("create = %d %s", created.Code, created.Body.String())
	}
	createdETag := created.Header().Get("ETag")
	listRequest := httptest.NewRequest("GET", "/api/v1/admin/docs", nil)
	listRequest.AddCookie(cookie)
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, listRequest)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), "managed.md") {
		t.Fatalf("list = %d %s", listed.Code, listed.Body.String())
	}
	detailRequest := httptest.NewRequest("GET", "/api/v1/admin/docs/managed.md", nil)
	detailRequest.AddCookie(cookie)
	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, detailRequest)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"description":"结构化说明"`) || !strings.Contains(detail.Body.String(), "title: 管理文档") {
		t.Fatalf("detail metadata = %d %s", detail.Code, detail.Body.String())
	}
	staleRequest := httptest.NewRequest("PUT", "/api/v1/admin/docs/managed.md", strings.NewReader(`{"content":"# 错误版本\n"}`))
	staleRequest.Header.Set("If-Match", `"stale"`)
	staleRequest.AddCookie(cookie)
	stale := httptest.NewRecorder()
	handler.ServeHTTP(stale, staleRequest)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale update = %d %s", stale.Code, stale.Body.String())
	}
	updateRequest := httptest.NewRequest("PUT", "/api/v1/admin/docs/managed.md", strings.NewReader(`{"content":"# 第二版\n","metadata":{"title":"第二版标题","description":"更新后的说明","tags":["更新"],"draft":false,"order":4}}`))
	updateRequest.Header.Set("If-Match", createdETag)
	updateRequest.AddCookie(cookie)
	updated := httptest.NewRecorder()
	handler.ServeHTTP(updated, updateRequest)
	if updated.Code != http.StatusOK || updated.Header().Get("ETag") == createdETag {
		t.Fatalf("update = %d %s", updated.Code, updated.Body.String())
	}
	deleteRequest := httptest.NewRequest("DELETE", "/api/v1/admin/docs/managed.md", nil)
	deleteRequest.Header.Set("If-Match", updated.Header().Get("ETag"))
	deleteRequest.AddCookie(cookie)
	deleted := httptest.NewRecorder()
	handler.ServeHTTP(deleted, deleteRequest)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestContentAPI(t *testing.T) {
	dataDir := t.TempDir()
	if err := ensureDataDirectories(dataDir); err != nil {
		t.Fatal(err)
	}
	contentDir := filepath.Join(dataDir, "content")
	if err := os.WriteFile(filepath.Join(contentDir, "welcome.md"), []byte("---\ntitle: 欢迎\ndescription: 第一篇文档\n---\n# 欢迎\n\n正文。\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "media", "logo.png"), []byte("png-data"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "media", "notes.html"), []byte("<script>bad</script>"), 0o640); err != nil {
		t.Fatal(err)
	}
	app := &server{
		config: config{DataDir: dataDir},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:  content.NewStore(contentDir),
	}
	handler := app.routes()

	treeResponse := httptest.NewRecorder()
	handler.ServeHTTP(treeResponse, httptest.NewRequest("GET", "/api/v1/tree", nil))
	if treeResponse.Code != 200 || !strings.Contains(treeResponse.Body.String(), "welcome.md") {
		t.Fatalf("tree response = %d %s", treeResponse.Code, treeResponse.Body.String())
	}

	documentResponse := httptest.NewRecorder()
	handler.ServeHTTP(documentResponse, httptest.NewRequest("GET", "/api/v1/docs/welcome.md", nil))
	var documentPayload content.Document
	if err := json.NewDecoder(documentResponse.Body).Decode(&documentPayload); err != nil {
		t.Fatal(err)
	}
	if documentResponse.Code != 200 || !strings.Contains(documentPayload.HTML, "<h1 id=\"欢迎\">欢迎</h1>") {
		t.Fatalf("document response = %d %s", documentResponse.Code, documentResponse.Body.String())
	}
	if documentResponse.Header().Get("ETag") == "" {
		t.Fatal("document response must include ETag")
	}
	documentNotModified := httptest.NewRecorder()
	documentConditionalRequest := httptest.NewRequest("GET", "/api/v1/docs/welcome.md", nil)
	documentConditionalRequest.Header.Set("If-None-Match", documentResponse.Header().Get("ETag"))
	handler.ServeHTTP(documentNotModified, documentConditionalRequest)
	if documentNotModified.Code != 304 || documentNotModified.Body.Len() != 0 {
		t.Fatalf("document conditional response = %d %q, want 304 without body", documentNotModified.Code, documentNotModified.Body.String())
	}

	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, httptest.NewRequest("GET", "/api/v1/docs/missing.md", nil))
	if missingResponse.Code != 404 {
		t.Fatalf("missing document status = %d, want 404", missingResponse.Code)
	}

	mediaResponse := httptest.NewRecorder()
	handler.ServeHTTP(mediaResponse, httptest.NewRequest("GET", "/media/logo.png", nil))
	if mediaResponse.Code != 200 || mediaResponse.Body.String() != "png-data" || mediaResponse.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("media response = %d %q %q", mediaResponse.Code, mediaResponse.Body.String(), mediaResponse.Header().Get("Content-Type"))
	}
	mediaNotModified := httptest.NewRecorder()
	mediaConditionalRequest := httptest.NewRequest("GET", "/media/logo.png", nil)
	mediaConditionalRequest.Header.Set("If-None-Match", mediaResponse.Header().Get("ETag"))
	handler.ServeHTTP(mediaNotModified, mediaConditionalRequest)
	if mediaNotModified.Code != 304 || mediaNotModified.Body.Len() != 0 {
		t.Fatalf("media conditional response = %d %q, want 304 without body", mediaNotModified.Code, mediaNotModified.Body.String())
	}

	attachmentResponse := httptest.NewRecorder()
	handler.ServeHTTP(attachmentResponse, httptest.NewRequest("GET", "/media/notes.html", nil))
	if attachmentResponse.Code != 200 || attachmentResponse.Header().Get("Content-Type") != "application/octet-stream" || attachmentResponse.Header().Get("Content-Disposition") == "" {
		t.Fatalf("attachment response = %d %q %q", attachmentResponse.Code, attachmentResponse.Header().Get("Content-Type"), attachmentResponse.Header().Get("Content-Disposition"))
	}

	traversalResponse := httptest.NewRecorder()
	app.media(traversalResponse, httptest.NewRequest("GET", "/media/../content/welcome.md", nil))
	if traversalResponse.Code != 404 {
		t.Fatalf("media traversal status = %d, want 404", traversalResponse.Code)
	}

	searchResponse := httptest.NewRecorder()
	handler.ServeHTTP(searchResponse, httptest.NewRequest("GET", "/api/v1/search?q=欢迎", nil))
	if searchResponse.Code != 200 || !strings.Contains(searchResponse.Body.String(), "welcome.md") {
		t.Fatalf("search response = %d %s", searchResponse.Code, searchResponse.Body.String())
	}
}
