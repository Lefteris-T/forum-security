package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	if server.MaxHeaderBytes != 1<<20 {
		t.Errorf(
			"MaxHeaderBytes = %d, want %d",
			server.MaxHeaderBytes,
			1<<20,
		)
	}
}
func writeTestCertificatePair(
	t *testing.T,
	directory string,
	name string,
) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test private key: %v", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore: now.Add(-time.Minute),
		NotAfter:  now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: []string{"localhost"},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
		},
	}

	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		t.Fatalf("create test certificate: %v", err)
	}

	certificatePath := filepath.Join(
		directory,
		name+".crt",
	)
	keyPath := filepath.Join(
		directory,
		name+".key",
	)

	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	if err := os.WriteFile(
		certificatePath,
		certificatePEM,
		0644,
	); err != nil {
		t.Fatalf("write test certificate: %v", err)
	}

	if err := os.WriteFile(
		keyPath,
		keyPEM,
		0600,
	); err != nil {
		t.Fatalf("write test private key: %v", err)
	}

	return certificatePath, keyPath
}
func TestLoadTLSCertificateAcceptsValidPair(t *testing.T) {
	directory := t.TempDir()
	certificatePath, keyPath := writeTestCertificatePair(
		t,
		directory,
		"valid",
	)

	certificate, err := loadTLSCertificate(
		certificatePath,
		keyPath,
	)
	if err != nil {
		t.Fatalf("loadTLSCertificate() error: %v", err)
	}

	if len(certificate.Certificate) == 0 {
		t.Error("loaded certificate chain is empty")
	}

	if certificate.PrivateKey == nil {
		t.Error("loaded private key is nil")
	}
}

func TestLoadTLSCertificateRejectsMissingCertificate(t *testing.T) {
	directory := t.TempDir()
	_, keyPath := writeTestCertificatePair(
		t,
		directory,
		"valid",
	)

	missingCertificate := filepath.Join(
		directory,
		"missing.crt",
	)

	_, err := loadTLSCertificate(
		missingCertificate,
		keyPath,
	)
	if err == nil {
		t.Fatal("loadTLSCertificate() error = nil, want an error")
	}
}

func TestLoadTLSCertificateRejectsMissingKey(t *testing.T) {
	directory := t.TempDir()
	certificatePath, _ := writeTestCertificatePair(
		t,
		directory,
		"valid",
	)

	missingKey := filepath.Join(
		directory,
		"missing.key",
	)

	_, err := loadTLSCertificate(
		certificatePath,
		missingKey,
	)
	if err == nil {
		t.Fatal("loadTLSCertificate() error = nil, want an error")
	}
}

func TestLoadTLSCertificateRejectsMalformedCertificate(t *testing.T) {
	directory := t.TempDir()

	certificatePath := filepath.Join(
		directory,
		"malformed.crt",
	)
	keyPath := filepath.Join(
		directory,
		"malformed.key",
	)

	if err := os.WriteFile(
		certificatePath,
		[]byte("not a certificate"),
		0644,
	); err != nil {
		t.Fatalf("write malformed certificate: %v", err)
	}

	if err := os.WriteFile(
		keyPath,
		[]byte("not a private key"),
		0600,
	); err != nil {
		t.Fatalf("write malformed key: %v", err)
	}

	_, err := loadTLSCertificate(
		certificatePath,
		keyPath,
	)
	if err == nil {
		t.Fatal("loadTLSCertificate() error = nil, want an error")
	}
}

func TestLoadTLSCertificateRejectsMismatchedPair(t *testing.T) {
	directory := t.TempDir()

	firstCertificate, _ := writeTestCertificatePair(
		t,
		directory,
		"first",
	)
	_, secondKey := writeTestCertificatePair(
		t,
		directory,
		"second",
	)

	_, err := loadTLSCertificate(
		firstCertificate,
		secondKey,
	)
	if err == nil {
		t.Fatal("loadTLSCertificate() error = nil, want an error")
	}
}
func TestLoadTLSCertificateDoesNotExposePrivateKeyContents(
	t *testing.T,
) {
	t.Parallel()

	directory := t.TempDir()

	certificatePath, keyPath := writeTestCertificatePair(
		t,
		directory,
		"secret",
	)

	const privateKeyContents = "private-key-super-secret-content"

	if err := os.WriteFile(
		keyPath,
		[]byte(privateKeyContents),
		0600,
	); err != nil {
		t.Fatalf("write malformed private key: %v", err)
	}

	_, err := loadTLSCertificate(
		certificatePath,
		keyPath,
	)
	if err == nil {
		t.Fatal("loadTLSCertificate() error = nil, want an error")
	}

	if strings.Contains(err.Error(), privateKeyContents) {
		t.Fatal("private-key contents leaked through startup error")
	}

	if !strings.Contains(
		err.Error(),
		"load TLS certificate and key",
	) {
		t.Errorf(
			"error lacks safe operation context: %q",
			err,
		)
	}
}
