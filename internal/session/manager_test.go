package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestManagerCreatesSessionCookie(t *testing.T) {
	manager := NewManager(
		"forum_session",
		24*time.Hour,
		false,
	)

	rec := httptest.NewRecorder()

	id, err := manager.Create(rec)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if id == "" {
		t.Fatal("session id is empty")
	}

	res := rec.Result()
	defer res.Body.Close()

	cookies := res.Cookies()

	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	cookie := cookies[0]

	if cookie.Name != "forum_session" {
		t.Fatalf(
			"cookie name = %q, want %q",
			cookie.Name,
			"forum_session",
		)
	}

	if cookie.Value != id {
		t.Fatalf(
			"cookie value = %q, want session id %q",
			cookie.Value,
			id,
		)
	}

	if !cookie.HttpOnly {
		t.Fatal("cookie HttpOnly = false, want true")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf(
			"cookie SameSite = %v, want Lax",
			cookie.SameSite,
		)
	}

	if cookie.Path != "/" {
		t.Fatalf("cookie Path = %q, want /", cookie.Path)
	}

	if cookie.Secure {
		t.Fatal("cookie Secure = true, want false")
	}
}
func TestManagerCreatesUniqueSessionIDs(t *testing.T) {
	manager := NewManager(
		"forum_session",
		24*time.Hour,
		false,
	)

	firstRecorder := httptest.NewRecorder()
	firstID, err := manager.Create(firstRecorder)
	if err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	secondRecorder := httptest.NewRecorder()
	secondID, err := manager.Create(secondRecorder)
	if err != nil {
		t.Fatalf("second Create() error: %v", err)
	}

	if firstID == secondID {
		t.Fatalf(
			"session IDs are equal: %q",
			firstID,
		)
	}
}
func TestManagerUsesSecureCookieWhenConfigured(t *testing.T) {
	manager := NewManager(
		"forum_session",
		24*time.Hour,
		true,
	)

	rec := httptest.NewRecorder()

	_, err := manager.Create(rec)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	res := rec.Result()
	defer res.Body.Close()

	cookies := res.Cookies()

	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	if !cookies[0].Secure {
		t.Fatal("cookie Secure = false, want true")
	}
}
func TestManagerClearsCookie(t *testing.T) {
	manager := NewManager(
		"forum_session",
		24*time.Hour,
		false,
	)

	rec := httptest.NewRecorder()

	manager.Clear(rec)

	res := rec.Result()
	defer res.Body.Close()

	cookies := res.Cookies()

	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	cookie := cookies[0]

	if cookie.Name != "forum_session" {
		t.Fatalf(
			"cookie name = %q, want %q",
			cookie.Name,
			"forum_session",
		)
	}

	if cookie.Value != "" {
		t.Fatalf(
			"cookie value = %q, want empty",
			cookie.Value,
		)
	}

	if cookie.MaxAge >= 0 {
		t.Fatalf(
			"cookie MaxAge = %d, want negative",
			cookie.MaxAge,
		)
	}
}
func TestManagerClearsSecureCookieWhenConfigured(t *testing.T) {
	manager := NewManager(
		"forum_session",
		time.Hour,
		true,
	)

	recorder := httptest.NewRecorder()

	manager.Clear(recorder)

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
