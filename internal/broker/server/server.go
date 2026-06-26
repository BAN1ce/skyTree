package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/BAN1ce/skyTree/internal/broker/server/tcp"
	"github.com/BAN1ce/skyTree/internal/broker/server/tlslistener"
	"github.com/BAN1ce/skyTree/internal/broker/server/wslistener"
	"github.com/BAN1ce/skyTree/logger"
	"net"
	"strings"
	"sync"
	"time"
)

type Listener interface {
	Accept() (net.Conn, error)
	Close() error
	Listen() error
	Name() string
}

type Server struct {
	listener []Listener
	mux      sync.RWMutex
	wg       sync.WaitGroup
	conn     chan net.Conn
	started  bool
	cancel   context.CancelFunc
	options  *serverOptions
}

func NewServer(adders []string, opts ...Option) (*Server, error) {
	var listener []Listener

	options := defaultServerOptions()
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, fmt.Errorf("apply server options error: %w", err)
		}
	}

	for _, address := range adders {
		protocol, addr, err := getProtocolAndAddress(address)
		if err != nil {
			return nil, fmt.Errorf("get protocol and address error: %w", err)
		}
		switch protocol {
		case "tcp":
			tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
			if err != nil {
				return nil, fmt.Errorf("resolve tcp addr error: %w", err)
			}
			listener = append(listener, tcp.NewListener(tcpAddr))

		case "tls":
			if options.tlsConfig == nil {
				return nil, fmt.Errorf("tls listener requires tls config (cert_file/key_file)")
			}
			tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
			if err != nil {
				return nil, fmt.Errorf("resolve tls addr error: %w", err)
			}
			listener = append(listener, tlslistener.NewListener(tcpAddr, options.tlsConfig))

		case "ws":
			tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
			if err != nil {
				return nil, fmt.Errorf("resolve ws addr error: %w", err)
			}
			listener = append(listener, wslistener.NewListener(tcpAddr, nil))

		case "wss":
			if options.tlsConfig == nil {
				return nil, fmt.Errorf("wss listener requires tls config (cert_file/key_file)")
			}
			tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
			if err != nil {
				return nil, fmt.Errorf("resolve wss addr error: %w", err)
			}
			listener = append(listener, wslistener.NewListener(tcpAddr, options.tlsConfig))

		default:
			return nil, fmt.Errorf("unsupported protocol: %s", protocol)
		}

	}

	return &Server{
		listener: listener,
		conn:     make(chan net.Conn),
		options:  options,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.started {
		return ErrServerStarted
	}
	if len(s.listener) == 0 {
		return ErrListenerIsNil
	}
	ctx, s.cancel = context.WithCancel(ctx)

	opened := make([]Listener, 0, len(s.listener))
	for _, l := range s.listener {
		if err := l.Listen(); err != nil {
			for _, openedListener := range opened {
				_ = openedListener.Close()
			}
			return fmt.Errorf("listen failed on %s: %w", l.Name(), err)
		}
		opened = append(opened, l)
		logger.Logger.Info().Str("Listener", l.Name()).Msg("listen success")
	}

	// Start TLS reloader if configured. It only affects tls:// listeners.
	if s.options != nil && s.options.tlsReloader != nil {
		s.options.tlsReloader.Start(ctx)
	}

	s.wg.Add(len(s.listener))
	s.startListener(ctx)
	s.started = true

	return nil
}

func (s *Server) startListener(ctx context.Context) {
	for _, l := range s.listener {
		go func(l Listener) {
			defer s.wg.Done()
			for {

				select {
				case <-ctx.Done():
					return
				default:
					conn, err := l.Accept()
					if err != nil {
						if errors.Is(err, net.ErrClosed) {
							logger.Logger.Info().Str("Listener", l.Name()).Msg("listener closed")
							return
						}
						// TODO: graceful shutdown should not output error loggers
						logger.Logger.Error().Err(err).Msg("accept error")
						continue
					}
					if conn != nil {
						select {
						case s.conn <- conn:
						case <-ctx.Done():
							_ = conn.Close()
							return
						}
					}
				}
			}
		}(l)
	}
}

func (s *Server) Close() error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if !s.started {
		return ErrServerNotStarted
	}
	if s.cancel != nil {
		s.cancel()
	}

	var closeErr error
	for _, l := range s.listener {
		logger.Logger.Info().Str("Listener", l.Name()).Msg("closing listener")
		if err := l.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close listener %s: %w", l.Name(), err))
		}
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	waitDone := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(waitDone)
	}()

	var waitErr error
	select {
	case <-waitDone:
	case <-waitCtx.Done():
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			waitErr = ErrCloseListenerTimeout
		}
	}
	s.started = false
	return errors.Join(closeErr, waitErr)
}

func (s *Server) Conn() <-chan net.Conn {
	return s.conn
}

func getProtocolAndAddress(address string) (string, string, error) {
	parts := strings.SplitN(address, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid address format: %s", address)
	}
	return parts[0], parts[1], nil
}
