package server

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"
)

type Option func(*serverOptions) error

type serverOptions struct {
	tlsConfig   *tls.Config
	tlsReloader *tlsReloader
}

func defaultServerOptions() *serverOptions {
	return &serverOptions{}
}

// WithTLSFiles enables server-side TLS for tls:// listeners by loading cert/key from disk.
// caFile is optional; when provided it is loaded into RootCAs for completeness/future mTLS.
func WithTLSFiles(certFile, keyFile, caFile string, reloadInterval time.Duration) Option {
	return WithTLSFilesAndMTLS(certFile, keyFile, caFile, reloadInterval, "")
}

// WithTLSFilesAndMTLS additionally configures mTLS client-cert behavior. mtlsAuthMode
// supports "off" (default) / "optional" / "required".
func WithTLSFilesAndMTLS(certFile, keyFile, caFile string, reloadInterval time.Duration, mtlsAuthMode string) Option {
	return func(o *serverOptions) error {
		mode := strings.TrimSpace(strings.ToLower(mtlsAuthMode))
		if certFile == "" && keyFile == "" && caFile == "" && reloadInterval <= 0 && (mode == "" || mode == "off") {
			return nil
		}
		if certFile == "" || keyFile == "" {
			return fmt.Errorf("tls cert_file and key_file are required")
		}
		reloader, tlsCfg, err := newTLSReloader(certFile, keyFile, caFile, reloadInterval)
		if err != nil {
			return err
		}
		if err := applyMTLSAuthMode(tlsCfg, caFile, mtlsAuthMode); err != nil {
			return err
		}
		o.tlsConfig = tlsCfg
		o.tlsReloader = reloader
		return nil
	}
}
