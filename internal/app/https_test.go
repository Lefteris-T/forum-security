package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"forum/internal/config"
)

func TestRunServesHTTPSAndShutsDown(t *testing.T) {
	directory := t.TempDir()

	certificatePath, keyPath := writeTestCertificatePair(
		t,
		directory,
		"localhost",
	)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve test address: %v", err)
	}

	address := listener.Addr().String()

	if err := listener.Close(); err != nil {
		t.Fatalf("release test address: %v", err)
	}

	cfg := config.Config{
		Address:         address,
		DatabasePath:    filepath.Join(directory, "forum.db"),
		SessionDuration: time.Hour,
		CookieName:      "forum_session",
		SecureCookie:    true,
		HTTPSEnabled:    true,
		TLSCertFile:     certificatePath,
		TLSKeyFile:      keyPath,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErrors := make(chan error, 1)

	go func() {
		runErrors <- Run(ctx, cfg)
	}()

	certificatePEM, err := os.ReadFile(certificatePath)
	if err != nil {
		t.Fatalf("read test certificate: %v", err)
	}

	rootCertificates := x509.NewCertPool()
	if !rootCertificates.AppendCertsFromPEM(certificatePEM) {
		t.Fatal("append test certificate to root pool")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    rootCertificates,
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
	}

	var response *http.Response
	var requestError error

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		response, requestError = client.Get(
			"https://" + address + "/",
		)
		if requestError == nil {
			break
		}

		select {
		case runError := <-runErrors:
			t.Fatalf(
				"Run() stopped before HTTPS request: %v",
				runError,
			)
		default:
		}

		time.Sleep(20 * time.Millisecond)
	}

	if requestError != nil {
		t.Fatalf("HTTPS request failed: %v", requestError)
	}

	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		response.Body.Close()
		t.Fatalf("read HTTPS response: %v", err)
	}

	if err := response.Body.Close(); err != nil {
		t.Fatalf("close HTTPS response: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf(
			"HTTPS status = %d, want %d",
			response.StatusCode,
			http.StatusOK,
		)
	}

	cancel()

	select {
	case runError := <-runErrors:
		if runError != nil {
			t.Fatalf(
				"Run() returned shutdown error: %v",
				runError,
			)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
}
