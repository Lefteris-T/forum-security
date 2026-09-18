package middleware

import (
	"log"
	"net/http"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader remembers the final status code for request logging.
func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}

	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return w.ResponseWriter.Write(body)
}

// RequestLogging records method, path, status, and elapsed request time.
func RequestLogging(
	logger *log.Logger,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{
			ResponseWriter: w,
		}

		next.ServeHTTP(sw, r)

		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}

		logger.Printf(
			"request method=%s path=%s status=%d",
			r.Method,
			r.URL.Path,
			status,
		)
	})
}
