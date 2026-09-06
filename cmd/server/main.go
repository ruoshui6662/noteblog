package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"markdown-docs/content"
	webassets "markdown-docs/web"
)

const serviceName = "markdown-docs"

type config struct {
	Address string
	DataDir string
}

type server struct {
	config config
	logger *slog.Logger
	store  *content.Store
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get(envOrDefault("APP_HEALTH_URL", "http://127.0.0.1:8080/readyz"))
		if err != nil {
			os.Exit(1)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	cfg := config{
		Address: envOrDefault("APP_ADDR", "127.0.0.1:8080"),
		DataDir: envOrDefault("APP_DATA_DIR", "var/data"),
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := ensureDataDirectories(cfg.DataDir); err != nil {
		logger.Error("failed to prepare data directory", "error", err)
		os.Exit(1)
	}

	store := content.NewStore(filepath.Join(cfg.DataDir, "content"))
	if err := store.Refresh(); err != nil {
		logger.Error("failed to scan content directory", "error", err)
		os.Exit(1)
	}
	app := &server{config: cfg, logger: logger, store: store}
	httpServer := &http.Server{
		Addr:              cfg.Address,
		Handler:           app.withRequestLog(app.routes()),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("server started", "service", serviceName, "address", cfg.Address, "data_dir", cfg.DataDir)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/v1/site", s.site)
	mux.HandleFunc("GET /api/v1/tree", s.tree)
	mux.HandleFunc("GET /api/v1/search", s.search)
	mux.HandleFunc("GET /api/v1/docs/", s.document)
	mux.HandleFunc("GET /media/", s.media)
	mux.HandleFunc("GET /api/", http.NotFound)
	mux.Handle("GET /", webassets.Handler())
	return mux
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) ready(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{"content", "media", "backups"} {
		info, err := os.Stat(filepath.Join(s.config.DataDir, name))
		if err != nil || !info.IsDir() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *server) site(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"name":        "Markdown 文档库",
		"stage":       "M1",
		"environment": envOrDefault("APP_ENV", "local"),
	})
}

func (s *server) tree(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "CONTENT_UNAVAILABLE", "内容服务未初始化")
		return
	}
	nodes, err := s.store.Tree()
	if err != nil {
		s.logger.Error("failed to build content tree", "error", err)
		writeError(w, http.StatusInternalServerError, "CONTENT_SCAN_FAILED", "无法读取文档目录")
		return
	}
	s.logContentIssues()
	writeJSON(w, http.StatusOK, nodes)
}

func (s *server) document(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "CONTENT_UNAVAILABLE", "内容服务未初始化")
		return
	}
	relativePath := strings.TrimPrefix(r.URL.Path, "/api/v1/docs/")
	document, found, err := s.store.Document(relativePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DOCUMENT_PATH", "文档路径无效")
		return
	}
	s.logContentIssues()
	if !found {
		writeError(w, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "文档不存在")
		return
	}
	etag := `"` + document.Hash + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (s *server) search(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "CONTENT_UNAVAILABLE", "内容服务未初始化")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > 100 {
		writeError(w, http.StatusBadRequest, "SEARCH_QUERY_TOO_LONG", "搜索词过长")
		return
	}
	results, err := s.store.Search(query)
	if err != nil {
		s.logger.Error("failed to search content", "error", err)
		writeError(w, http.StatusInternalServerError, "CONTENT_SCAN_FAILED", "无法搜索文档")
		return
	}
	s.logContentIssues()
	writeJSON(w, http.StatusOK, results)
}

func (s *server) media(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.TrimPrefix(r.URL.Path, "/media/")
	mediaPath, err := content.ResolveMediaPath(filepath.Join(s.config.DataDir, "media"), relativePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.logger.Warn("media request rejected", "path", relativePath, "error", err)
		}
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(mediaPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	etag := fmt.Sprintf("\"%x-%x\"", info.Size(), info.ModTime().UnixNano())
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if isInlineImage(mediaPath) {
		if contentType := mime.TypeByExtension(filepath.Ext(mediaPath)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(mediaPath)}))
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func isInlineImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif":
		return true
	default:
		return false
	}
}

func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}

func (s *server) logContentIssues() {
	for _, issue := range s.store.Issues() {
		s.logger.Warn("content file excluded", "path", issue.Path, "error", issue.Message)
	}
}

func (s *server) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started).Round(time.Millisecond))
	})
}

func ensureDataDirectories(dataDir string) error {
	for _, name := range []string{"content", "media", "backups"} {
		if err := os.MkdirAll(filepath.Join(dataDir, name), 0o750); err != nil {
			return err
		}
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
