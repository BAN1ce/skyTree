package tcp

import (
	"net"
)

type Listener struct {
	listener net.Listener
	addr     *net.TCPAddr
}

func NewListener(addr *net.TCPAddr) *Listener {
	var listener = &Listener{}
	return listener

}

func (l *Listener) Accept() (net.Conn, error) {
	return l.listener.Accept()
}

func (l *Listener) Close() error {
	return l.listener.Close()
}

func (l *Listener) Listen() error {
	var (
		err error
	)
	l.listener, err = net.ListenTCP("tcp", l.addr)
	if err != nil {
		return err
	}
	return err
}

func (l *Listener) Name() string {
	return l.listener.Addr().String()
}
