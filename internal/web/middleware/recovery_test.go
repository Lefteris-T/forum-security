package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryTurnsPanicIntoSafe500(t *testing.T) {
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
		panic("database password=super-secret")
	})

	h := Recovery(
		logger,
		next,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/panic",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusInternalServerError,
		)
	}

	if strings.Contains(
		rec.Body.String(),
		"super-secret",
	) {
		t.Fatal("panic details leaked to response")
	}
	wantBody := http.StatusText(
		http.StatusInternalServerError,
	) + "\n"

	if rec.Body.String() != wantBody {
		t.Errorf(
			"body = %q, want %q",
			rec.Body.String(),
			wantBody,
		)
	}

	output := logs.String()

	if strings.Contains(output, "super-secret") {
		t.Fatal("panic details leaked to logs")
	}

	for _, expected := range []string{
		"panic recovered",
		"method=GET",
		"path=/panic",
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
func TestRecoveryAllowsFollowingRequest(t *testing.T) {
	var logs bytes.Buffer

	logger := log.New(
		&logs,
		"",
		0,
	)

	calls := 0

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		calls++

		if calls == 1 {
			panic("boom")
		}

		w.WriteHeader(http.StatusNoContent)
	})

	h := Recovery(
		logger,
		next,
	)

	firstReq := httptest.NewRequest(
		http.MethodGet,
		"/panic",
		nil,
	)

	firstRec := httptest.NewRecorder()

	h.ServeHTTP(
		firstRec,
		firstReq,
	)

	if firstRec.Code != http.StatusInternalServerError {
		t.Fatalf(
			"first status = %d, want %d",
			firstRec.Code,
			http.StatusInternalServerError,
		)
	}

	secondReq := httptest.NewRequest(
		http.MethodGet,
		"/ok",
		nil,
	)

	secondRec := httptest.NewRecorder()

	h.ServeHTTP(
		secondRec,
		secondReq,
	)

	if secondRec.Code != http.StatusNoContent {
		t.Fatalf(
			"second status = %d, want %d",
			secondRec.Code,
			http.StatusNoContent,
		)
	}
}
