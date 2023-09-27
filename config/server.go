package config

type Server struct {
	Port       int
	BrokerPort int
}

func GetServer() Server {
	// FIXME:  use flag at here  not graceful
	return Server{
		Port:       *httpPort,
		BrokerPort: *brokerPort,
	}
}

func (e Server) GetPort() int {
	return e.Port
}

func (e Server) GetBrokerPort() int {
	return e.BrokerPort
}
