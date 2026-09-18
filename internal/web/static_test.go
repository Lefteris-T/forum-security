package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticFileHandlerDisablesDirectoryListings(
	t *testing.T,
) {
	t.Parallel()

	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")

	if err := os.MkdirAll(uploads, 0755); err != nil {
		t.Fatalf("create uploads directory: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(root, "style.css"),
		[]byte("body {}"),
		0644,
	); err != nil {
		t.Fatalf("write stylesheet: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(uploads, "image.jpg"),
		[]byte("image-bytes"),
		0644,
	); err != nil {
		t.Fatalf("write uploaded image: %v", err)
	}

	handler := NewStaticFileHandler(root)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "root directory listing",
			path:       "/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "uploads directory listing",
			path:       "/uploads/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "stylesheet remains public",
			path:       "/style.css",
			wantStatus: http.StatusOK,
			wantBody:   "body {}",
		},
		{
			name:       "uploaded image remains public",
			path:       "/uploads/image.jpg",
			wantStatus: http.StatusOK,
			wantBody:   "image-bytes",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(
				http.MethodGet,
				tt.path,
				nil,
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					tt.wantStatus,
				)
			}

			if tt.wantBody != "" &&
				rec.Body.String() != tt.wantBody {
				t.Errorf(
					"body = %q, want %q",
					rec.Body.String(),
					tt.wantBody,
				)
			}
		})
	}
}
