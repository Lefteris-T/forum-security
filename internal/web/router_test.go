package web

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMethodHandlerRejectsWrongMethodWithAllowHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := methodHandler(
		[]string{http.MethodPost},
		next,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/posts",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusMethodNotAllowed,
		)
	}

	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf(
			"Allow = %q, want %q",
			got,
			http.MethodPost,
		)
	}
}
func TestMethodHandlerAllowsExpectedMethod(t *testing.T) {
	called := false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	h := methodHandler(
		[]string{http.MethodPost},
		next,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/posts",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}

	if !called {
		t.Fatal("next handler was not called")
	}
}
func TestWriteErrorRendersSafeStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"bad request", http.StatusBadRequest},
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"not found", http.StatusNotFound},
		{"method not allowed", http.StatusMethodNotAllowed},
		{"conflict", http.StatusConflict},
		{"too many requests", http.StatusTooManyRequests},
		{"internal server error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeError(
				rec,
				tt.status,
			)

			if rec.Code != tt.status {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					tt.status,
				)
			}

			if !strings.Contains(
				rec.Body.String(),
				http.StatusText(tt.status),
			) {
				t.Fatalf(
					"body %q does not contain %q",
					rec.Body.String(),
					http.StatusText(tt.status),
				)
			}
		})
	}
}
func TestWriteErrorDoesNotExposeInternalDetails(t *testing.T) {
	rec := httptest.NewRecorder()

	writeError(
		rec,
		http.StatusInternalServerError,
	)

	body := rec.Body.String()

	if strings.Contains(
		body,
		"database",
	) {
		t.Fatal("internal details leaked in response")
	}
}
func TestNewRouterRegistersRouteWithMethod(t *testing.T) {
	called := false

	h := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	router := NewRouter([]Route{
		{
			Methods: []string{http.MethodGet},
			Pattern: "/test",
			Handler: h,
		},
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}

	if !called {
		t.Fatal("registered handler was not called")
	}
}
func TestNewRouterRejectsWrongMethod(t *testing.T) {
	h := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		t.Fatal("handler should not be called")
	})

	router := NewRouter([]Route{
		{
			Methods: []string{http.MethodPost},
			Pattern: "/test",
			Handler: h,
		},
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusMethodNotAllowed,
		)
	}

	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf(
			"Allow = %q, want %q",
			got,
			http.MethodPost,
		)
	}
}
func TestMethodHandlerAllowsMultipleMethods(t *testing.T) {
	called := 0

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		called++
		w.WriteHeader(http.StatusNoContent)
	})

	h := methodHandler(
		[]string{
			http.MethodGet,
			http.MethodPost,
		},
		next,
	)

	for _, method := range []string{
		http.MethodGet,
		http.MethodPost,
	} {
		req := httptest.NewRequest(
			method,
			"/login",
			nil,
		)

		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf(
				"%s status = %d, want %d",
				method,
				rec.Code,
				http.StatusNoContent,
			)
		}
	}

	if called != 2 {
		t.Fatalf(
			"handler called %d times, want 2",
			called,
		)
	}
}
func TestMethodHandlerListsAllAllowedMethods(t *testing.T) {
	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		t.Fatal("handler should not be called")
	})

	h := methodHandler(
		[]string{
			http.MethodGet,
			http.MethodPost,
		},
		next,
	)

	req := httptest.NewRequest(
		http.MethodDelete,
		"/login",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusMethodNotAllowed,
		)
	}

	if got := rec.Header().Get("Allow"); got != "GET, POST" {
		t.Fatalf(
			"Allow = %q, want %q",
			got,
			"GET, POST",
		)
	}
}
func TestPostRoutesDispatchesPostDetail(t *testing.T) {
	detailCalled := false

	detail := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		detailCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	h := postRoutes(
		detail,
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/posts/42",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}

	if !detailCalled {
		t.Fatal("post detail handler was not called")
	}
}
func TestPostRoutesDispatchesCommentCreation(t *testing.T) {
	commentCalled := false

	comment := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		commentCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	h := postRoutes(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		comment,
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/posts/42/comments",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}

	if !commentCalled {
		t.Fatal("comment handler was not called")
	}
}
func TestPostRoutesDispatchesPostReaction(t *testing.T) {
	reactionCalled := false

	reaction := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		reactionCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	h := postRoutes(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		reaction,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/posts/42/react",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}

	if !reactionCalled {
		t.Fatal("post reaction handler was not called")
	}
}
func TestCommentRoutesDispatchesCommentReaction(t *testing.T) {
	called := false

	reaction := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	h := commentRoutes(reaction)

	req := httptest.NewRequest(
		http.MethodPost,
		"/comments/42/react",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}

	if !called {
		t.Fatal("comment reaction handler was not called")
	}
}
func TestCommentRoutesRejectsWrongMethod(t *testing.T) {
	reaction := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		t.Fatal("handler should not be called")
	})

	h := commentRoutes(reaction)

	req := httptest.NewRequest(
		http.MethodGet,
		"/comments/42/react",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusMethodNotAllowed,
		)
	}

	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf(
			"Allow = %q, want %q",
			got,
			http.MethodPost,
		)
	}
}
func TestCommentRoutesReturnsNotFoundForUnknownPath(t *testing.T) {
	h := commentRoutes(
		http.HandlerFunc(func(
			http.ResponseWriter,
			*http.Request,
		) {
			t.Fatal("handler should not be called")
		}),
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/comments/42/whatever",
		nil,
	)

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNotFound,
		)
	}
}
func TestForumRouterMethods(t *testing.T) {
	okHandler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusNoContent)
	})

	router := NewForumRouter(Handlers{
		Home:            okHandler,
		Register:        okHandler,
		Login:           okHandler,
		Logout:          okHandler,
		PostCreation:    okHandler,
		PostDetail:      okHandler,
		CommentCreate:   okHandler,
		PostReaction:    okHandler,
		CommentReaction: okHandler,
	})

	tests := []struct {
		name   string
		method string
		path   string
		want   int
		allow  string
	}{
		{
			name:   "home get",
			method: http.MethodGet,
			path:   "/",
			want:   http.StatusNoContent,
		},
		{
			name:   "home post rejected",
			method: http.MethodPost,
			path:   "/",
			want:   http.StatusMethodNotAllowed,
			allow:  http.MethodGet,
		},
		{
			name:   "register get",
			method: http.MethodGet,
			path:   "/register",
			want:   http.StatusNoContent,
		},
		{
			name:   "register post",
			method: http.MethodPost,
			path:   "/register",
			want:   http.StatusNoContent,
		},
		{
			name:   "login get",
			method: http.MethodGet,
			path:   "/login",
			want:   http.StatusNoContent,
		},
		{
			name:   "login post",
			method: http.MethodPost,
			path:   "/login",
			want:   http.StatusNoContent,
		},
		{
			name:   "logout post",
			method: http.MethodPost,
			path:   "/logout",
			want:   http.StatusNoContent,
		},
		{
			name:   "logout get rejected",
			method: http.MethodGet,
			path:   "/logout",
			want:   http.StatusMethodNotAllowed,
			allow:  http.MethodPost,
		},
		{
			name:   "new post get",
			method: http.MethodGet,
			path:   "/posts/new",
			want:   http.StatusNoContent,
		},
		{
			name:   "create post",
			method: http.MethodPost,
			path:   "/posts",
			want:   http.StatusNoContent,
		},
		{
			name:   "post detail",
			method: http.MethodGet,
			path:   "/posts/42",
			want:   http.StatusNoContent,
		},
		{
			name:   "post detail wrong method",
			method: http.MethodDelete,
			path:   "/posts/42",
			want:   http.StatusMethodNotAllowed,
			allow:  http.MethodGet,
		},
		{
			name:   "comment create",
			method: http.MethodPost,
			path:   "/posts/42/comments",
			want:   http.StatusNoContent,
		},
		{
			name:   "comment create get rejected",
			method: http.MethodGet,
			path:   "/posts/42/comments",
			want:   http.StatusMethodNotAllowed,
			allow:  http.MethodPost,
		},
		{
			name:   "post reaction",
			method: http.MethodPost,
			path:   "/posts/42/react",
			want:   http.StatusNoContent,
		},
		{
			name:   "post reaction get rejected",
			method: http.MethodGet,
			path:   "/posts/42/react",
			want:   http.StatusMethodNotAllowed,
			allow:  http.MethodPost,
		},
		{
			name:   "comment reaction",
			method: http.MethodPost,
			path:   "/comments/42/react",
			want:   http.StatusNoContent,
		},
		{
			name:   "comment reaction get rejected",
			method: http.MethodGet,
			path:   "/comments/42/react",
			want:   http.StatusMethodNotAllowed,
			allow:  http.MethodPost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				tt.method,
				tt.path,
				nil,
			)

			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					tt.want,
				)
			}

			if tt.allow != "" {
				if got := rec.Header().Get("Allow"); got != tt.allow {
					t.Fatalf(
						"Allow = %q, want %q",
						got,
						tt.allow,
					)
				}
			}
		})
	}
}
func TestWriteErrorRendersForbidden(t *testing.T) {
	rec := httptest.NewRecorder()

	writeError(
		rec,
		http.StatusForbidden,
	)

	if rec.Code != http.StatusForbidden {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusForbidden,
		)
	}

	if !strings.Contains(
		rec.Body.String(),
		http.StatusText(http.StatusForbidden),
	) {
		t.Fatal("forbidden status text was not rendered")
	}
}
func TestForumRouterRejectsGETOnMutationRoutes(t *testing.T) {
	mutationHandler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		t.Fatal("mutation handler must not run for GET")
	})

	okHandler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusNoContent)
	})

	router := NewForumRouter(Handlers{
		Home:            okHandler,
		Register:        okHandler,
		Login:           okHandler,
		Logout:          mutationHandler,
		PostCreation:    mutationHandler,
		PostDetail:      okHandler,
		CommentCreate:   mutationHandler,
		PostReaction:    mutationHandler,
		CommentReaction: mutationHandler,
	})

	tests := []struct {
		path  string
		allow string
	}{
		{
			path:  "/logout",
			allow: http.MethodPost,
		},
		{
			path:  "/posts",
			allow: http.MethodPost,
		},
		{
			path:  "/posts/42/comments",
			allow: http.MethodPost,
		},
		{
			path:  "/posts/42/react",
			allow: http.MethodPost,
		},
		{
			path:  "/comments/42/react",
			allow: http.MethodPost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				tt.path,
				nil,
			)

			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					http.StatusMethodNotAllowed,
				)
			}

			if got := rec.Header().Get("Allow"); got != tt.allow {
				t.Fatalf(
					"Allow = %q, want %q",
					got,
					tt.allow,
				)
			}
		})
	}
}
func TestMiddlewareStackRecoversAndKeepsServing(t *testing.T) {
	var logs bytes.Buffer

	logger := log.New(
		&logs,
		"",
		0,
	)

	calls := 0

	handler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		calls++

		if calls == 1 {
			panic("secret panic data")
		}

		w.WriteHeader(http.StatusNoContent)
	})

	h := WithMiddleware(
		logger,
		handler,
	)

	firstReq := httptest.NewRequest(
		http.MethodGet,
		"/panic",
		nil,
	)

	firstRec := httptest.NewRecorder()

	h.ServeHTTP(firstRec, firstReq)

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

	h.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusNoContent {
		t.Fatalf(
			"second status = %d, want %d",
			secondRec.Code,
			http.StatusNoContent,
		)
	}

	if strings.Contains(
		logs.String(),
		"secret panic data",
	) {
		t.Fatal("panic secret leaked to logs")
	}
}
func TestForumRouterServesStaticCSS(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(dir, "style.css"),
		[]byte("body { margin: 0; }"),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	okHandler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusNoContent)
	})

	router := NewForumRouter(Handlers{
		Home:            okHandler,
		Register:        okHandler,
		Login:           okHandler,
		Logout:          okHandler,
		PostCreation:    okHandler,
		PostDetail:      okHandler,
		CommentCreate:   okHandler,
		PostReaction:    okHandler,
		CommentReaction: okHandler,
		Static:          http.FileServer(http.Dir(dir)),
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/static/style.css",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusOK,
		)
	}

	if !strings.Contains(
		rec.Body.String(),
		"body",
	) {
		t.Fatal("CSS file was not served")
	}
}
func TestForumRouterGitHubOAuthCallbackRoute(t *testing.T) {
	called := false

	callback := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	router := NewForumRouter(Handlers{
		GitHubOAuthCallback: callback,
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/github/callback",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if !called {
		t.Fatal("GitHub OAuth callback handler was not called")
	}

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}
}

func TestForumRouterGoogleOAuthRoutes(t *testing.T) {
	startCalled := false
	callbackCalled := false

	start := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		startCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	callback := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		callbackCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	router := NewForumRouter(Handlers{
		GoogleOAuth:         start,
		GoogleOAuthCallback: callback,
	})

	for _, path := range []string{
		"/auth/google",
		"/auth/google/callback",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf(
				"%s status = %d, want %d",
				path,
				rec.Code,
				http.StatusNoContent,
			)
		}
	}

	if !startCalled {
		t.Fatal("Google OAuth start handler was not called")
	}

	if !callbackCalled {
		t.Fatal("Google OAuth callback handler was not called")
	}
}

func TestForumRouterOmitsGoogleOAuthRoutesWhenDisabled(t *testing.T) {
	router := NewForumRouter(Handlers{})

	for _, path := range []string{
		"/auth/google",
		"/auth/google/callback",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf(
				"%s status = %d, want %d",
				path,
				rec.Code,
				http.StatusNotFound,
			)
		}
	}
}
