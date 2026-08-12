package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"forge/internal/config"
	"forge/internal/database"
	"forge/internal/http/middleware"
	"forge/internal/http/response"
	"forge/internal/logger"
	"forge/internal/version"
)

type HealthResponse struct {
	Status string `json:"status"`
}

type VersionResponse struct {
	Version string `json:"version"`
}

func healthHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())

		log.Info("health check",
			"method", r.Method,
			"path", r.URL.Path,
			"request_id", requestID,
		)

		responseBody := HealthResponse{
			Status: "ok",
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(responseBody); err != nil {
			log.Error("failed to encode health response",
				"error", err,
				"request_id", requestID,
			)

			response.JSONError(
				w,
				http.StatusInternalServerError,
				"internal server error",
			)

			return
		}
	}
}

func readyHandler(db database.Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			response.JSONError(
				w,
				http.StatusServiceUnavailable,
				"database unavailable",
			)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(HealthResponse{
			Status: "ok",
		}); err != nil {
			response.JSONError(
				w,
				http.StatusInternalServerError,
				"internal server error",
			)
		}
	}
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	responseBody := VersionResponse{
		Version: version.Value,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(responseBody); err != nil {
		response.JSONError(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	response.JSONError(
		w,
		http.StatusNotFound,
		"not found",
	)
}

func main() {
	cfg := config.Load()
	log := logger.New()

	dbContext, dbCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer dbCancel()

	db, err := database.Connect(dbContext, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database",
			"error", err,
		)
		os.Exit(1)
	}
	defer db.Close()

	log.Info("database connection established")

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler(log))
	mux.HandleFunc("/ready", readyHandler(db))
	mux.HandleFunc("/version", versionHandler)
	mux.HandleFunc("/", notFoundHandler)

	handler := middleware.SecurityHeaders(mux)
	handler = middleware.BodyLimit(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.RequestLogger(log)(handler)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Info("Forge API starting",
		"port", cfg.Port,
		"version", version.Value,
	)

	shutdownSignals := make(chan os.Signal, 1)

	signal.Notify(
		shutdownSignals,
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer signal.Stop(shutdownSignals)

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped unexpectedly",
				"error", err,
			)
			os.Exit(1)
		}

		return

	case signal := <-shutdownSignals:
		log.Info("shutdown signal received",
			"signal", signal.String(),
		)
	}

	shutdownContext, shutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer shutdownCancel()

	log.Info("starting graceful shutdown")

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Error("graceful shutdown failed",
			"error", err,
		)

		return
	}

	log.Info("graceful shutdown completed")

	log.Info("Forge API stopped")
}
