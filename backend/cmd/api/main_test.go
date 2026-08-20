package main

import (
	"bytes"
	"context"
	"encoding/json"
	"forge/internal/database"
	"forge/internal/http/middleware"
	"forge/internal/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
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

func TestLoginProductionHTTPPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://forge:forge_dev_password@localhost:5432/forge"
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	email := "task28-" + uuid.NewString() + "@example.com"
	password := "correct horse battery staple"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		_, _ = pool.Exec(
			cleanupCtx,
			`DELETE FROM users WHERE email = $1`,
			email,
		)
	})

	userRepository := database.NewUserRepository(pool)
	userService := users.NewService(userRepository)

	if _, err := userService.Register(
		ctx,
		email,
		password,
	); err != nil {
		t.Fatalf("register test user: %v", err)
	}

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := newHTTPHandler(logger, pool, false)

	t.Run("valid credentials", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"/login",
			bytes.NewBufferString(
				`{"email":"`+email+`","password":"`+password+`"}`,
			),
		)
		request.Header.Set("Content-Type", "application/json")

		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf(
				"expected status %d, got %d",
				http.StatusOK,
				response.Code,
			)
		}

		var body struct {
			ID       uuid.UUID `json:"id"`
			Email    string    `json:"email"`
			Password string    `json:"password"`
		}

		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if body.ID == uuid.Nil {
			t.Fatal("expected authenticated user ID")
		}

		if body.Email != email {
			t.Fatalf(
				"expected email %q, got %q",
				email,
				body.Email,
			)
		}

		if body.Password != "" {
			t.Fatal("response must not contain password")
		}

		if bytes.Contains(
			response.Body.Bytes(),
			[]byte("$2a$"),
		) {
			t.Fatal("response must not contain password hash")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"/login",
			bytes.NewBufferString(
				`{"email":"`+email+`","password":"wrong password"}`,
			),
		)
		request.Header.Set("Content-Type", "application/json")

		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf(
				"expected status %d, got %d",
				http.StatusUnauthorized,
				response.Code,
			)
		}

		if bytes.Contains(
			response.Body.Bytes(),
			[]byte("password"),
		) {
			t.Fatal("response must not expose password details")
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"/login",
			bytes.NewBufferString(
				`{"email":"missing-`+uuid.NewString()+`@example.com","password":"wrong password"}`,
			),
		)
		request.Header.Set("Content-Type", "application/json")

		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf(
				"expected status %d, got %d",
				http.StatusUnauthorized,
				response.Code,
			)
		}

		expected := `{"error":"invalid credentials"}` + "\n"

		if response.Body.String() != expected {
			t.Fatalf(
				"expected body %q, got %q",
				expected,
				response.Body.String(),
			)
		}
	})

	t.Run("repository failure remains internal", func(t *testing.T) {
		pool.Close()

		request := httptest.NewRequest(
			http.MethodPost,
			"/login",
			bytes.NewBufferString(
				`{"email":"`+email+`","password":"`+password+`"}`,
			),
		)
		request.Header.Set("Content-Type", "application/json")

		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusInternalServerError {
			t.Fatalf(
				"expected status %d, got %d",
				http.StatusInternalServerError,
				response.Code,
			)
		}

		expected := `{"error":"internal server error"}` + "\n"

		if response.Body.String() != expected {
			t.Fatalf(
				"expected body %q, got %q",
				expected,
				response.Body.String(),
			)
		}

		if bytes.Contains(
			response.Body.Bytes(),
			[]byte("database unavailable"),
		) {
			t.Fatal("response must not expose database error details")
		}

		if bytes.Contains(
			response.Body.Bytes(),
			[]byte("password"),
		) {
			t.Fatal("response must not expose password details")
		}
	})
}
