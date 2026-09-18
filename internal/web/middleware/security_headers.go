package middleware

import "net/http"

// SecurityHeaders adds baseline browser security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.Header().Set(
			"X-Content-Type-Options",
			"nosniff",
		)
		w.Header().Set(
			"X-Frame-Options",
			"DENY",
		)
		w.Header().Set(
			"Referrer-Policy",
			"same-origin",
		)

		next.ServeHTTP(w, r)
	})
}
