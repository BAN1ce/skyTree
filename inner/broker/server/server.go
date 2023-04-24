package server

import (
	"context"
	"errors"
	"github.com/BAN1ce/skyTree/inner/broker/server/tcp"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"net"
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
}

func NewTCPServer(addr *net.TCPAddr) *Server {
	return NewServer([]Listener{tcp.NewListener(addr)})
}

func NewServer(listener []Listener) *Server {
	return &Server{
		listener: listener,
		conn:     make(chan net.Conn),
	}
}

func (s *Server) Start() error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.started {
		return errs.ErrServerStarted
	}
	if len(s.listener) == 0 {
		return errs.ErrListenerIsNil
	}
	s.wg.Add(len(s.listener))
	for _, l := range s.listener {
		go func(l Listener) {
			defer s.wg.Done()
			if err := l.Listen(); err != nil {
				logger.Logger.Fatalln("listen error = ", err.Error())
			}
			logger.Logger.Info("listen success = ", l.Name())
			for {
				if con, err := l.Accept(); err != nil {
					logger.Logger.Error("accept error = ", err.Error())
					return
				} else {
					logger.Logger.Debug("accept success = ", con.RemoteAddr().String())
					s.conn <- con
				}
			}
			logger.Logger.WithField("Listener", l.Name()).Info("listen close")
		}(l)
	}
	s.started = true
	return nil
}

func (s *Server) Close() error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if !s.started {
		return errs.ErrServerNotStarted
	}
	for _, l := range s.listener {
		if err := l.Close(); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	go func() {
		s.wg.Wait()
		cancel()
	}()
	<-ctx.Done()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errs.ErrCloseListenerTimeout
	}
	s.started = false
	return nil
}

func (s *Server) Conn() (net.Conn, bool) {
	// TODO: channel close panic
	conn, ok := <-s.conn
	return conn, ok
}
