package config

type Server struct {
	Port       int
	BrokerPort int
}

func GetServer() Server {
	return Server{
		Port:       9526,
		BrokerPort: 1883,
	}
}

func (e Server) GetPort() int {
	return e.Port
}

func (e Server) GetBrokerPort() int {
	return e.BrokerPort
}
