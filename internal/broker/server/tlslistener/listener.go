package tlslistener

import (
	"crypto/tls"
	"net"
)

type Listener struct {
	listener net.Listener
	addr     *net.TCPAddr
	tlsCfg   *tls.Config
}

func NewListener(addr *net.TCPAddr, tlsCfg *tls.Config) *Listener {
	return &Listener{
		addr:   addr,
		tlsCfg: tlsCfg,
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	return l.listener.Accept()
}

func (l *Listener) Close() error {
	return l.listener.Close()
}

func (l *Listener) Listen() error {
	base, err := net.ListenTCP("tcp", l.addr)
	if err != nil {
		return err
	}
	l.listener = tls.NewListener(base, l.tlsCfg)
	return nil
}

func (l *Listener) Name() string {
	if l.listener == nil {
		return l.addr.String()
	}
	return l.listener.Addr().String()
}
