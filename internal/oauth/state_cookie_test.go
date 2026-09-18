package oauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWriteStateCookie(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteStateCookie(
		rec,
		"oauth_state",
		"state-value",
		false,
	)

	res := rec.Result()
	defer res.Body.Close()

	cookies := res.Cookies()

	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	cookie := cookies[0]

	if cookie.Name != "oauth_state" {
		t.Errorf("cookie.Name = %q, want %q", cookie.Name, "oauth_state")
	}

	if cookie.Value != "state-value" {
		t.Errorf("cookie.Value = %q, want %q", cookie.Value, "state-value")
	}

	if !cookie.HttpOnly {
		t.Error("cookie.HttpOnly = false, want true")
	}

	if cookie.Secure {
		t.Error("cookie.Secure = true, want false")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf(
			"cookie.SameSite = %v, want %v",
			cookie.SameSite,
			http.SameSiteLaxMode,
		)
	}

	if cookie.Path != "/" {
		t.Errorf("cookie.Path = %q, want %q", cookie.Path, "/")
	}

	if cookie.MaxAge <= 0 {
		t.Errorf("cookie.MaxAge = %d, want positive value", cookie.MaxAge)
	}
}

func TestWriteStateCookieUsesSecureFlag(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteStateCookie(
		rec,
		"oauth_state",
		"state-value",
		true,
	)

	res := rec.Result()
	defer res.Body.Close()

	cookies := res.Cookies()

	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	if !cookies[0].Secure {
		t.Error("cookie.Secure = false, want true")
	}
}

func TestClearStateCookie(t *testing.T) {
	rec := httptest.NewRecorder()

	ClearStateCookie(
		rec,
		"oauth_state",
		false,
	)

	res := rec.Result()
	defer res.Body.Close()

	cookies := res.Cookies()

	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	cookie := cookies[0]

	if cookie.Value != "" {
		t.Errorf("cookie.Value = %q, want empty", cookie.Value)
	}

	if cookie.MaxAge >= 0 {
		t.Errorf("cookie.MaxAge = %d, want negative", cookie.MaxAge)
	}

	if cookie.Expires.After(time.Now()) {
		t.Errorf("cookie.Expires = %v, want expired cookie", cookie.Expires)
	}

	if !cookie.HttpOnly {
		t.Error("cookie.HttpOnly = false, want true")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf(
			"cookie.SameSite = %v, want %v",
			cookie.SameSite,
			http.SameSiteLaxMode,
		)
	}
}
func TestClearStateCookieUsesSecureFlag(t *testing.T) {
	recorder := httptest.NewRecorder()

	ClearStateCookie(
		recorder,
		"oauth_state",
		true,
	)

	response := recorder.Result()
	defer response.Body.Close()

	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf(
			"cookie count = %d, want 1",
			len(cookies),
		)
	}

	if !cookies[0].Secure {
		t.Error("cleared cookie Secure = false, want true")
	}
}
