package wslistener

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

type Listener struct {
	addr      *net.TCPAddr
	tlsCfg    *tls.Config
	listener  net.Listener
	server    *http.Server
	connCh    chan net.Conn
	done      chan struct{}
	closeOnce sync.Once
}

func NewListener(addr *net.TCPAddr, tlsCfg *tls.Config) *Listener {
	return &Listener{
		addr:   addr,
		tlsCfg: tlsCfg,
		connCh: make(chan net.Conn),
		done:   make(chan struct{}),
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connCh:
		if conn == nil {
			return nil, net.ErrClosed
		}
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *Listener) Close() error {
	var err error
	l.closeOnce.Do(func() {
		close(l.done)
		if l.server != nil {
			err = errors.Join(err, l.server.Close())
		}
		if l.listener != nil {
			err = errors.Join(err, l.listener.Close())
		}
	})
	return err
}

func (l *Listener) Listen() error {
	base, err := net.ListenTCP("tcp", l.addr)
	if err != nil {
		return err
	}
	l.listener = base
	if l.tlsCfg != nil {
		l.listener = tls.NewListener(base, l.tlsCfg)
	}

	mux := http.NewServeMux()
	wsServer := websocket.Server{
		Handshake: func(cfg *websocket.Config, _ *http.Request) error {
			for _, protocol := range cfg.Protocol {
				if protocol == "mqtt" {
					cfg.Protocol = []string{"mqtt"}
					return nil
				}
			}
			return errors.New("websocket mqtt subprotocol required")
		},
		Handler: func(conn *websocket.Conn) {
			wrapped := &connWithDone{
				Conn: conn,
				done: make(chan struct{}),
			}
			defer wrapped.Close()

			select {
			case l.connCh <- wrapped:
			case <-l.done:
				return
			}

			select {
			case <-wrapped.done:
			case <-l.done:
			}
		},
	}
	mux.Handle("/mqtt", wsServer)
	mux.Handle("/", wsServer)

	l.server = &http.Server{Handler: mux}
	go func() {
		err := l.server.Serve(l.listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			_ = l.Close()
		}
	}()
	return nil
}

func (l *Listener) Name() string {
	if l.listener == nil {
		return l.addr.String()
	}
	return l.listener.Addr().String()
}

type connWithDone struct {
	*websocket.Conn
	done chan struct{}
	once sync.Once
}

func (c *connWithDone) Close() error {
	var err error
	c.once.Do(func() {
		close(c.done)
		err = c.Conn.Close()
	})
	return err
}
