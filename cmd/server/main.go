package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	app := &server{config: cfg, logger: logger}
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
		"stage":       "M0",
		"environment": envOrDefault("APP_ENV", "local"),
	})
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
