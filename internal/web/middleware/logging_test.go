package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggingLogsSafeContext(t *testing.T) {
	var logs bytes.Buffer

	logger := log.New(
		&logs,
		"",
		0,
	)

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusNoContent)
	})

	h := RequestLogging(
		logger,
		next,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/posts/42",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	output := logs.String()

	if !strings.Contains(output, "method=GET") {
		t.Fatal("method was not logged")
	}

	if !strings.Contains(output, "path=/posts/42") {
		t.Fatal("path was not logged")
	}

	if !strings.Contains(output, "status=204") {
		t.Fatal("status was not logged")
	}
}
func TestRequestLoggingDoesNotLogSecrets(t *testing.T) {
	var logs bytes.Buffer

	logger := log.New(
		&logs,
		"",
		0,
	)

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusOK)
	})

	h := RequestLogging(
		logger,
		next,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login?token=super-secret",
		strings.NewReader(
			"email=a@example.com&password=my-secret-password",
		),
	)

	req.Header.Set(
		"Authorization",
		"Bearer secret-token",
	)

	req.Header.Set(
		"Cookie",
		"session=secret-session-id",
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	output := logs.String()

	secrets := []string{
		"super-secret",
		"my-secret-password",
		"secret-token",
		"secret-session-id",
	}

	for _, secret := range secrets {
		if strings.Contains(output, secret) {
			t.Fatalf(
				"secret %q leaked to logs",
				secret,
			)
		}
	}
}
func TestRequestLoggingRecordsTooManyRequests(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	logger := log.New(
		&logs,
		"",
		0,
	)

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		w.Header().Set("Retry-After", "12")

		http.Error(
			w,
			http.StatusText(http.StatusTooManyRequests),
			http.StatusTooManyRequests,
		)
	})

	handler := RequestLogging(logger, next)

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusTooManyRequests,
		)
	}

	output := logs.String()

	for _, expected := range []string{
		"method=POST",
		"path=/login",
		"status=429",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf(
				"log does not contain %q: %q",
				expected,
				output,
			)
		}
	}
}
func TestRequestLoggingKeepsFirstWrittenStatus(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	logger := log.New(
		&logs,
		"",
		0,
	)

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError)
	})

	handler := RequestLogging(logger, next)

	req := httptest.NewRequest(
		http.MethodPost,
		"/posts",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf(
			"response status = %d, want %d",
			rec.Code,
			http.StatusCreated,
		)
	}

	output := logs.String()

	if !strings.Contains(output, "status=201") {
		t.Errorf(
			"log does not contain first status: %q",
			output,
		)
	}

	if strings.Contains(output, "status=500") {
		t.Errorf(
			"log contains status not sent to client: %q",
			output,
		)
	}
}
