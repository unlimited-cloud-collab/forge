package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"forge/internal/http/middleware"
)

type mockPinger struct {
	err error
}

func (m mockPinger) Ping(context.Context) error {
	return m.err
}

func TestHealthHandler(t *testing.T) {
	logger := testLogger()

	handler := healthHandler(logger)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	expected := `{"status":"ok"}` + "\n"

	if response.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, response.Body.String())
	}
}

func TestReadyHandler(t *testing.T) {
	handler := readyHandler(mockPinger{})

	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	expected := `{"status":"ok"}` + "\n"

	if response.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, response.Body.String())
	}
}

func TestReadyHandlerDatabaseUnavailable(t *testing.T) {
	expectedError := context.DeadlineExceeded

	handler := readyHandler(mockPinger{
		err: expectedError,
	})

	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			response.Code,
		)
	}

	expected := `{"error":"database unavailable"}` + "\n"

	if response.Body.String() != expected {
		t.Fatalf(
			"expected body %q, got %q",
			expected,
			response.Body.String(),
		)
	}
}

func TestVersionHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/version", nil)
	response := httptest.NewRecorder()

	versionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	expected := `{"version":"0.1.0"}` + "\n"

	if response.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, response.Body.String())
	}
}

func TestNotFoundHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	response := httptest.NewRecorder()

	notFoundHandler(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNotFound,
			response.Code,
		)
	}

	expected := `{"error":"not found"}` + "\n"

	if response.Body.String() != expected {
		t.Fatalf(
			"expected body %q, got %q",
			expected,
			response.Body.String(),
		)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())

		if requestID == "" {
			t.Fatal("expected request ID in context")
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := middleware.RequestID(next)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNoContent,
			response.Code,
		)
	}

	requestID := response.Header().Get("X-Request-ID")

	if requestID == "" {
		t.Fatal("expected X-Request-ID response header")
	}
}

func TestRequestIDMiddlewarePreservesExistingRequestID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.RequestIDFromContext(r.Context())

		if requestID != "test-request-123" {
			t.Fatalf(
				"expected request ID %q, got %q",
				"test-request-123",
				requestID,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := middleware.RequestID(next)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "test-request-123")

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Header().Get("X-Request-ID") != "test-request-123" {
		t.Fatalf(
			"expected response request ID %q, got %q",
			"test-request-123",
			response.Header().Get("X-Request-ID"),
		)
	}
}

func TestRequestLogger(t *testing.T) {
	var logs bytes.Buffer

	logger := slog.New(
		slog.NewJSONHandler(&logs, nil),
	)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := middleware.RequestID(
		middleware.RequestLogger(logger)(next),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/test",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	requestID := response.Header().Get("X-Request-ID")

	if requestID == "" {
		t.Fatal("expected X-Request-ID response header")
	}

	var logEntry map[string]any

	if err := json.Unmarshal(logs.Bytes(), &logEntry); err != nil {
		t.Fatalf(
			"expected valid JSON log entry, got %q: %v",
			logs.String(),
			err,
		)
	}

	if logEntry["msg"] != "http request" {
		t.Fatalf(
			"expected log message %q, got %q",
			"http request",
			logEntry["msg"],
		)
	}

	if logEntry["method"] != http.MethodPost {
		t.Fatalf(
			"expected method %q, got %q",
			http.MethodPost,
			logEntry["method"],
		)
	}

	if logEntry["path"] != "/test" {
		t.Fatalf(
			"expected path %q, got %q",
			"/test",
			logEntry["path"],
		)
	}

	if logEntry["status"] != float64(http.StatusCreated) {
		t.Fatalf(
			"expected status %d, got %v",
			http.StatusCreated,
			logEntry["status"],
		)
	}

	duration, ok := logEntry["duration_ms"].(float64)

	if !ok {
		t.Fatalf(
			"expected duration_ms to be numeric, got %T",
			logEntry["duration_ms"],
		)
	}

	if duration < 0 {
		t.Fatalf(
			"expected duration_ms to be non-negative, got %v",
			duration,
		)
	}

	if logEntry["request_id"] != requestID {
		t.Fatalf(
			"expected request_id %q, got %q",
			requestID,
			logEntry["request_id"],
		)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
