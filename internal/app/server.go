package app

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

const (
	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 2 * time.Minute
	serverWriteTimeout      = 2 * time.Minute
	serverIdleTimeout       = 60 * time.Second
	serverMaxHeaderBytes    = 64 << 10
)

func newHTTPServer(
	address string,
	handler http.Handler,
	tlsConfig *tls.Config,
) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
		MaxHeaderBytes:    serverMaxHeaderBytes,
	}
}
func loadTLSCertificate(
	certificateFile string,
	keyFile string,
) (tls.Certificate, error) {
	certificate, err := tls.LoadX509KeyPair(
		certificateFile,
		keyFile,
	)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf(
			"load TLS certificate and key: %w",
			err,
		)
	}

	return certificate, nil
}
