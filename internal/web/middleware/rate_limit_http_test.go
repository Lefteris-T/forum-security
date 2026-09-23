package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientKeyNormalizesRemoteAddress(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{
			name:       "IPv4",
			remoteAddr: "192.0.2.10:54321",
			want:       "192.0.2.10",
		},
		{
			name:       "IPv6",
			remoteAddr: "[2001:db8::1]:54321",
			want:       "2001:db8::1",
		},
		{
			name:       "IPv4 mapped IPv6",
			remoteAddr: "[::ffff:192.0.2.10]:54321",
			want:       "192.0.2.10",
		},
		{
			name:       "missing port",
			remoteAddr: "192.0.2.10",
			want:       unknownClientKey,
		},
		{
			name:       "invalid IP",
			remoteAddr: "not-an-ip:54321",
			want:       unknownClientKey,
		},
		{
			name:       "empty address",
			remoteAddr: "",
			want:       unknownClientKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				"https://localhost/",
				nil,
			)
			request.RemoteAddr = tt.remoteAddr

			got := clientKey(request)

			if got != tt.want {
				t.Errorf(
					"clientKey() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestClientKeyIgnoresForwardingHeaders(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"https://localhost/",
		nil,
	)
	request.RemoteAddr = "192.0.2.10:54321"

	request.Header.Set(
		"X-Forwarded-For",
		"203.0.113.50",
	)
	request.Header.Set(
		"X-Real-IP",
		"203.0.113.60",
	)

	got := clientKey(request)

	if got != "192.0.2.10" {
		t.Errorf(
			"clientKey() = %q, want direct peer %q",
			got,
			"192.0.2.10",
		)
	}
}
func TestRequestRateLimitRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		want   rateLimitRule
	}{
		{
			name:   "login POST",
			method: http.MethodPost,
			path:   "/login",
			want:   rateLimitRuleLogin,
		},
		{
			name:   "register POST",
			method: http.MethodPost,
			path:   "/register",
			want:   rateLimitRuleRegister,
		},
		{
			name:   "post creation",
			method: http.MethodPost,
			path:   "/posts",
			want:   rateLimitRulePost,
		},
		{
			name:   "comment creation",
			method: http.MethodPost,
			path:   "/posts/42/comments",
			want:   rateLimitRuleComment,
		},
		{
			name:   "post reaction",
			method: http.MethodPost,
			path:   "/posts/42/react",
			want:   rateLimitRuleReaction,
		},
		{
			name:   "comment reaction",
			method: http.MethodPost,
			path:   "/comments/15/react",
			want:   rateLimitRuleReaction,
		},
		{
			name:   "query string does not affect rule",
			method: http.MethodPost,
			path:   "/login?next=/posts",
			want:   rateLimitRuleLogin,
		},
		{
			name:   "GET is not specifically limited",
			method: http.MethodGet,
			path:   "/login",
			want:   rateLimitRuleNone,
		},
		{
			name:   "logout has no specific rule",
			method: http.MethodPost,
			path:   "/logout",
			want:   rateLimitRuleNone,
		},
		{
			name:   "unknown POST route",
			method: http.MethodPost,
			path:   "/unknown",
			want:   rateLimitRuleNone,
		},
		{
			name:   "incomplete comment route",
			method: http.MethodPost,
			path:   "/posts/comments",
			want:   rateLimitRuleNone,
		},
		{
			name:   "extra path segment",
			method: http.MethodPost,
			path:   "/posts/42/comments/extra",
			want:   rateLimitRuleNone,
		},
		{
			name:   "trailing slash does not match",
			method: http.MethodPost,
			path:   "/login/",
			want:   rateLimitRuleNone,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, tt.path, nil)

			if got := requestRateLimitRule(req); got != tt.want {
				t.Fatalf(
					"requestRateLimitRule() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}
func TestNewHTTPRateLimiterCreatesAllPolicies(t *testing.T) {
	t.Parallel()

	limiter, err := NewHTTPRateLimiter()
	if err != nil {
		t.Fatalf("NewHTTPRateLimiter() error = %v", err)
	}
	t.Cleanup(limiter.Stop)

	if limiter.global == nil {
		t.Fatal("global limiter is nil")
	}

	rules := []rateLimitRule{
		rateLimitRuleLogin,
		rateLimitRuleRegister,
		rateLimitRulePost,
		rateLimitRuleComment,
		rateLimitRuleReaction,
	}

	if got := len(limiter.byRule); got != len(rules) {
		t.Fatalf(
			"len(byRule) = %d, want %d",
			got,
			len(rules),
		)
	}

	for _, rule := range rules {
		if limiter.byRule[rule] == nil {
			t.Errorf("limiter for rule %q is nil", rule)
		}
	}
}
func TestHTTPRateLimiterStopStopsAllLimiters(t *testing.T) {
	t.Parallel()

	limiter, err := NewHTTPRateLimiter()
	if err != nil {
		t.Fatalf("NewHTTPRateLimiter() error = %v", err)
	}

	limiter.Stop()
	limiter.Stop()

	assertStopped := func(name string, value *RateLimiter) {
		t.Helper()

		select {
		case <-value.doneCh:
		case <-time.After(time.Second):
			t.Fatalf("%s limiter did not stop", name)
		}
	}

	assertStopped("global", limiter.global)

	for rule, ruleLimiter := range limiter.byRule {
		assertStopped(string(rule), ruleLimiter)
	}
}
func TestHTTPRateLimiterEnforcesGlobalLimit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	limiter, err := newHTTPRateLimiter(func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("newHTTPRateLimiter() error = %v", err)
	}
	t.Cleanup(limiter.Stop)

	nextCalls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	})

	handler := limiter.Middleware(next)

	for i := 0; i < 120; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.10:4567"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf(
				"request %d status = %d, want %d",
				i+1,
				rec.Code,
				http.StatusNoContent,
			)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:4567"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf(
			"limited status = %d, want %d",
			rec.Code,
			http.StatusTooManyRequests,
		)
	}

	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want %q", got, "1")
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want HTML", got)
	}
	if !strings.Contains(rec.Body.String(), `href="/"`) {
		t.Fatal("global limit page does not link back to the forum")
	}

	if nextCalls != 120 {
		t.Errorf("next calls = %d, want 120", nextCalls)
	}
}
func TestHTTPRateLimiterEnforcesSpecificRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		allowed    int
		retryAfter string
		returnPath string
	}{
		{"login", "/login", 5, "12", "/login"},
		{"register", "/register", 5, "12", "/register"},
		{"post", "/posts", 20, "3", "/"},
		{"comment", "/posts/42/comments", 30, "2", "/"},
		{"post reaction", "/posts/42/react", 60, "1", "/"},
		{"comment reaction", "/comments/15/react", 60, "1", "/"},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			now := time.Date(
				2026,
				1,
				1,
				12,
				0,
				0,
				0,
				time.UTC,
			)

			limiter, err := newHTTPRateLimiter(func() time.Time {
				return now
			})
			if err != nil {
				t.Fatalf("newHTTPRateLimiter() error = %v", err)
			}
			t.Cleanup(limiter.Stop)

			nextCalls := 0
			next := http.HandlerFunc(func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				nextCalls++
				w.WriteHeader(http.StatusNoContent)
			})

			handler := limiter.Middleware(next)

			for i := 0; i < tt.allowed; i++ {
				req := httptest.NewRequest(
					http.MethodPost,
					tt.path,
					nil,
				)
				req.RemoteAddr = "203.0.113.10:4567"

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusNoContent {
					t.Fatalf(
						"request %d status = %d, want %d",
						i+1,
						rec.Code,
						http.StatusNoContent,
					)
				}
			}

			req := httptest.NewRequest(
				http.MethodPost,
				tt.path,
				nil,
			)
			req.RemoteAddr = "203.0.113.10:4567"

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf(
					"limited status = %d, want %d",
					rec.Code,
					http.StatusTooManyRequests,
				)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "Too Many Requests") {
				t.Error("limited response does not explain the error")
			}
			if !strings.Contains(body, `href="`+tt.returnPath+`"`) {
				t.Errorf("limited response does not link to %q", tt.returnPath)
			}
			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want HTML", got)
			}

			if got := rec.Header().Get("Retry-After"); got != tt.retryAfter {
				t.Errorf(
					"Retry-After = %q, want %q",
					got,
					tt.retryAfter,
				)
			}

			if nextCalls != tt.allowed {
				t.Errorf(
					"next calls = %d, want %d",
					nextCalls,
					tt.allowed,
				)
			}
		})
	}
}

func TestHTTPRateLimiterCountsSuccessfulAndFailedLoginsTogether(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter, err := newHTTPRateLimiter(func() time.Time { return now })
	if err != nil {
		t.Fatalf("newHTTPRateLimiter() error = %v", err)
	}
	t.Cleanup(limiter.Stop)

	responses := []int{
		http.StatusSeeOther,
		http.StatusUnauthorized,
		http.StatusSeeOther,
		http.StatusUnauthorized,
		http.StatusSeeOther,
	}
	nextCalls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(responses[nextCalls])
		nextCalls++
	})
	handler := limiter.Middleware(next)

	for requestNumber, wantStatus := range responses {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.10:4567"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != wantStatus {
			t.Fatalf(
				"request %d status = %d, want %d",
				requestNumber+1,
				rec.Code,
				wantStatus,
			)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "203.0.113.10:4567"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("sixth login status = %d, want 429", rec.Code)
	}
	if nextCalls != len(responses) {
		t.Fatalf("next calls = %d, want %d", nextCalls, len(responses))
	}
}

func TestHTTPRateLimiterKeepsClientAllowancesIndependent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter, err := newHTTPRateLimiter(func() time.Time { return now })
	if err != nil {
		t.Fatalf("newHTTPRateLimiter() error = %v", err)
	}
	t.Cleanup(limiter.Stop)

	handler := limiter.Middleware(http.HandlerFunc(func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.10:4567"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("first client request %d status = %d", i+1, rec.Code)
		}
	}

	blocked := httptest.NewRequest(http.MethodPost, "/login", nil)
	blocked.RemoteAddr = "203.0.113.10:4567"
	blockedRec := httptest.NewRecorder()
	handler.ServeHTTP(blockedRec, blocked)
	if blockedRec.Code != http.StatusTooManyRequests {
		t.Fatalf("exhausted client status = %d, want 429", blockedRec.Code)
	}

	independent := httptest.NewRequest(http.MethodPost, "/login", nil)
	independent.RemoteAddr = "203.0.113.11:4567"
	independentRec := httptest.NewRecorder()
	handler.ServeHTTP(independentRec, independent)
	if independentRec.Code != http.StatusNoContent {
		t.Fatalf(
			"independent client status = %d, want %d",
			independentRec.Code,
			http.StatusNoContent,
		)
	}
}

func TestHTTPRateLimiterKeepsLoginAndRegistrationRulesIndependent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter, err := newHTTPRateLimiter(func() time.Time { return now })
	if err != nil {
		t.Fatalf("newHTTPRateLimiter() error = %v", err)
	}
	t.Cleanup(limiter.Stop)

	handler := limiter.Middleware(http.HandlerFunc(func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.10:4567"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("login request %d status = %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/register", nil)
	req.RemoteAddr = "203.0.113.10:4567"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"registration status = %d, want independent allowance",
			rec.Code,
		)
	}
}
