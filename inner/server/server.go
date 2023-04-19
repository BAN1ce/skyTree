package server

import (
	"context"
	"errors"
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

type server struct {
	listener []Listener
	mux      sync.RWMutex
	wg       sync.WaitGroup
	started  bool
}

func NewServer(listener []Listener) *server {
	return &server{
		listener: listener,
	}
}

func (s *server) Start() error {
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
			logger.Logger.WithField("Listener", l.Name()).Info("listen close")
		}(l)
	}
	s.started = true
	return nil
}

func (s *server) Close() error {
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
