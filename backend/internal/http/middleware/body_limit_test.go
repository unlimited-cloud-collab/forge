package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimitAllowsRequestsWithinLimit(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		if string(body) != "hello" {
			t.Fatalf("expected body %q, got %q", "hello", string(body))
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := BodyLimit(next)

	req := httptest.NewRequest(
		http.MethodPost,
		"/test",
		strings.NewReader("hello"),
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNoContent,
			rec.Code,
		)
	}
}

func TestBodyLimitRejectsRequestsOverLimit(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected body read to fail")
		}

		if err.Error() == "" {
			t.Fatal("expected body read error")
		}

		w.WriteHeader(http.StatusRequestEntityTooLarge)
	})

	handler := BodyLimit(next)

	body := strings.Repeat("a", int(MaxRequestBodySize)+1)

	req := httptest.NewRequest(
		http.MethodPost,
		"/test",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusRequestEntityTooLarge,
			rec.Code,
		)
	}
}
