package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := SecurityHeaders(next)

	req := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
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

	tests := []struct {
		name     string
		expected string
		actual   string
	}{
		{
			name:     "content type options",
			expected: "nosniff",
			actual:   rec.Header().Get("X-Content-Type-Options"),
		},
		{
			name:     "frame options",
			expected: "DENY",
			actual:   rec.Header().Get("X-Frame-Options"),
		},
		{
			name:     "referrer policy",
			expected: "no-referrer",
			actual:   rec.Header().Get("Referrer-Policy"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.actual != test.expected {
				t.Fatalf(
					"expected %q, got %q",
					test.expected,
					test.actual,
				)
			}
		})
	}
}
