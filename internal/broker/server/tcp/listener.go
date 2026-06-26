package tcp

import (
	"net"
)

type Listener struct {
	listener net.Listener
	addr     *net.TCPAddr
}

func NewListener(addr *net.TCPAddr) *Listener {
	var listener = &Listener{
		addr: addr,
	}
	return listener

}

func (l *Listener) Accept() (net.Conn, error) {
	return l.listener.Accept()
}

func (l *Listener) Close() error {
	return l.listener.Close()
}

func (l *Listener) Listen() error {
	listener, err := net.ListenTCP("tcp", l.addr)
	if err != nil {
		return err
	}
	l.listener = listener
	return nil
}

func (l *Listener) Name() string {
	if l.listener == nil {
		return l.addr.String()
	}
	return l.listener.Addr().String()
}
