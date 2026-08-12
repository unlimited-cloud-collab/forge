package middleware

import "net/http"

const MaxRequestBodySize int64 = 1 << 20 // 1 MiB

func BodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(
			w,
			r.Body,
			MaxRequestBodySize,
		)

		next.ServeHTTP(w, r)
	})
}
