package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/BAN1ce/skyTree/logger"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestDoReceivedConnectStopsOnFirstError(t *testing.T) {
	logger.LoadForTest()

	calledSecond := false
	expectedErr := errors.New("boom")
	p := &Plugins{
		PacketPlugin: PacketPlugin{
			OnReceivedConnect: []OnReceivedConnect{
				func(ctx context.Context, clientID string, connect *packets.Connect) error {
					return expectedErr
				},
				func(ctx context.Context, clientID string, connect *packets.Connect) error {
					calledSecond = true
					return nil
				},
			},
		},
	}

	err := p.DoReceivedConnect(context.Background(), "c1", &packets.Connect{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if calledSecond {
		t.Fatal("expected second hook not called after first error")
	}
}

func TestDoReceivedAuthNoHooksReturnsInput(t *testing.T) {
	logger.LoadForTest()

	in := &packets.Auth{ReasonCode: packets.AuthSuccess}
	p := &Plugins{}
	out, err := p.DoReceivedAuth(context.Background(), "c1", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != in {
		t.Fatal("expected original auth packet when no hooks are registered")
	}
}

func TestDoReceivedAuthRejectsNilPluginResult(t *testing.T) {
	logger.LoadForTest()

	p := &Plugins{
		PacketPlugin: PacketPlugin{
			OnReceivedAuth: []OnReceivedAuth{
				func(ctx context.Context, clientID string, auth *packets.Auth) (*packets.Auth, error) {
					return nil, nil
				},
			},
		},
	}

	out, err := p.DoReceivedAuth(context.Background(), "c1", &packets.Auth{ReasonCode: packets.AuthSuccess})
	if err == nil {
		t.Fatal("expected error when plugin returns nil auth packet")
	}
	if out != nil {
		t.Fatal("expected nil output when plugin returns nil auth packet")
	}
}
