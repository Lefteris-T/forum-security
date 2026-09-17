package app

import (
	"crypto/tls"
	"testing"
)

func TestNewTLSConfigUsesTLS12Minimum(t *testing.T) {
	cfg := newTLSConfig()

	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf(
			"MinVersion = %d, want TLS 1.2 (%d)",
			cfg.MinVersion,
			tls.VersionTLS12,
		)
	}
}

func TestNewTLSConfigUsesApprovedTLS12CipherSuites(t *testing.T) {
	cfg := newTLSConfig()

	approved := map[uint16]bool{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:       true,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:         true,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:       true,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:         true,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256: true,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256:   true,
	}

	if len(cfg.CipherSuites) != len(approved) {
		t.Fatalf(
			"CipherSuites contains %d suites, want %d",
			len(cfg.CipherSuites),
			len(approved),
		)
	}

	for _, suite := range cfg.CipherSuites {
		if !approved[suite] {
			t.Errorf("CipherSuites contains unapproved suite: %d", suite)
		}
	}
}

func TestNewTLSConfigDoesNotDisableTLS13(t *testing.T) {
	cfg := newTLSConfig()

	if cfg.MaxVersion != 0 && cfg.MaxVersion < tls.VersionTLS13 {
		t.Errorf(
			"MaxVersion = %d, want zero or at least TLS 1.3 (%d)",
			cfg.MaxVersion,
			tls.VersionTLS13,
		)
	}
}
