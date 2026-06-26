package config

import "time"

// TLS holds server-side TLS configuration.
// It is shared by broker listeners (tls://, wss://), HTTP API, and optional gRPC.
type TLS struct {
	Enabled bool `yaml:"enabled" env:"TLS_ENABLED" env-default:"false"`

	// CertFile and KeyFile are PEM files for the server certificate and private key.
	CertFile string `yaml:"cert_file" env:"TLS_CERT_FILE" env-default:""`
	KeyFile  string `yaml:"key_file" env:"TLS_KEY_FILE" env-default:""`

	// CAFile is optional. For server-side TLS-only it is not required, but is kept
	// for future mTLS/client cert validation or full chain verification.
	CAFile string `yaml:"ca_file" env:"TLS_CA_FILE" env-default:""`

	// ReloadInterval enables periodic reload of cert/key from disk when > 0.
	// SkyTree uses watch + poll (this interval) to rotate certificates without restart.
	ReloadInterval time.Duration `yaml:"reload_interval" env:"TLS_RELOAD_INTERVAL" env-default:"0s"`

	// MTLSAuthMode controls client certificate handling.
	//   "off"      —— do not request client certificate (default).
	//   "optional" —— request but do not require; auth plugins MAY use peer cert if present.
	//   "required" —— require & verify client certificate against CAFile; fail handshake otherwise.
	MTLSAuthMode string `yaml:"mtls_auth_mode" env:"TLS_MTLS_AUTH_MODE" env-default:"off"`
}
