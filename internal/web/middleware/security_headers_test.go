package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersSetsRequiredHeaders(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := SecurityHeaders(next)

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}

	tests := []struct {
		name string
		want string
	}{
		{
			name: "X-Content-Type-Options",
			want: "nosniff",
		},
		{
			name: "X-Frame-Options",
			want: "DENY",
		},
		{
			name: "Referrer-Policy",
			want: "same-origin",
		},
	}

	for _, tt := range tests {
		if got := rec.Header().Get(tt.name); got != tt.want {
			t.Errorf(
				"%s = %q, want %q",
				tt.name,
				got,
				tt.want,
			)
		}
	}
}
