package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// Item represents a resource managed by the API.
type Item struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	items   = make(map[string]Item)
	itemsMu sync.RWMutex
	nextID  int
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(),
	}))
	slog.SetDefault(logger)

	mux := http.NewServeMux()

	// Middleware: structured request logging.
	handler := loggingMiddleware(mux)

	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/items", handleListItems)
	mux.HandleFunc("POST /api/items", handleCreateItem)

	addr := listenAddr()
	slog.Info("server starting", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

// handleHealth returns the service health status.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "go-web-service",
	})
}

// handleListItems returns all items.
func handleListItems(w http.ResponseWriter, _ *http.Request) {
	itemsMu.RLock()
	defer itemsMu.RUnlock()

	list := make([]Item, 0, len(items))
	for _, item := range items {
		list = append(list, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// handleCreateItem adds a new item.
func handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	itemsMu.Lock()
	nextID++
	item := Item{
		ID:        fmt.Sprintf("item_%d", nextID),
		Name:      req.Name,
		CreatedAt: time.Now().UTC(),
	}
	items[item.ID] = item
	itemsMu.Unlock()

	slog.Info("item created", "id", item.ID, "name", item.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(item)
}

// loggingMiddleware logs every incoming HTTP request with structured fields.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.statusCode = code
	sw.ResponseWriter.WriteHeader(code)
}

// listenAddr returns the server listen address from the environment or a default.
func listenAddr() string {
	if addr := os.Getenv("LISTEN_ADDR"); addr != "" {
		return addr
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

// logLevel returns the slog level based on the LOG_LEVEL environment variable.
func logLevel() slog.Level {
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
