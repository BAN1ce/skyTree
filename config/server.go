package config

type Server struct {
	Port int `yaml:"port" env:"SERVER_PORT" env-default:"9526"`

	// TLS is used by HTTPS API server when enabled.
	TLS TLS `yaml:"tls"`
}

func (e Server) GetPort() int {
	return e.Port
}
