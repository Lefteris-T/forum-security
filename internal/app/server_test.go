package app

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerAppliesSecurityConfiguration(t *testing.T) {
	handler := http.NewServeMux()
	tlsConfig := newTLSConfig()

	server := newHTTPServer(
		"127.0.0.1:8443",
		handler,
		tlsConfig,
	)

	if server.Addr != "127.0.0.1:8443" {
		t.Errorf(
			"Addr = %q, want %q",
			server.Addr,
			"127.0.0.1:8443",
		)
	}

	if server.Handler != handler {
		t.Error("Handler was not preserved")
	}

	if server.TLSConfig != tlsConfig {
		t.Error("TLSConfig was not preserved")
	}

	timeoutTests := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{
			name: "read header timeout",
			got:  server.ReadHeaderTimeout,
			want: 5 * time.Second,
		},
		{
			name: "read timeout",
			got:  server.ReadTimeout,
			want: 2 * time.Minute,
		},
		{
			name: "write timeout",
			got:  server.WriteTimeout,
			want: 2 * time.Minute,
		},
		{
			name: "idle timeout",
			got:  server.IdleTimeout,
			want: 60 * time.Second,
		},
	}

	for _, tt := range timeoutTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf(
					"timeout = %v, want %v",
					tt.got,
					tt.want,
				)
			}
		})
	}

	if server.MaxHeaderBytes != 64<<10 {
		t.Errorf(
			"MaxHeaderBytes = %d, want %d",
			server.MaxHeaderBytes,
			64<<10,
		)
	}
}
