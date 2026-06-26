package client

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	delivery_event "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/eventbus"
	"github.com/rs/zerolog"
)

func TestStartClientDeliveryRunner_NoDeadlockUnderClientMux(t *testing.T) {
	// InnerHandler.HandlePacket holds client.mux.Lock() and then calls StartClientDeliveryRunner().
	// If StartClientDeliveryRunner (or its once.Do closure) calls exported getters like GetID(), it may re-lock c.mux and deadlock.

	// Initialize a minimal logger without depending on loading ../../../etc/config.yaml.
	// SkyLogger has unexported fields; only set the embedded zerolog.Logger.
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	localEvent := eventbus.NewEventCenter[*delivery_event.Notify]()
	ev := delivery_notify.New(1, localEvent, nil)

	cl := NewClient(c1, WithClientDeliveryEvent(ev))
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

	done := make(chan struct{})
	go func() {
		cl.mux.Lock()
		cl.StartClientDeliveryRunner()
		cl.mux.Unlock()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(300 * time.Millisecond):
		t.Fatal("StartClientDeliveryRunner appears to deadlock when called under client.mux.Lock()")
	}
}
