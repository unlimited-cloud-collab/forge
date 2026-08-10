package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"forge/internal/http/middleware"
)

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
	logger := testLogger()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := middleware.RequestID(
		middleware.RequestLogger(logger)(next),
	)

	request := httptest.NewRequest(http.MethodPost, "/test", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID response header")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}