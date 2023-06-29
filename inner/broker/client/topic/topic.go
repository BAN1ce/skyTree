package topic

import (
	"context"
	"github.com/eclipse/paho.golang/packets"
)

type Topic interface {
	Start(ctx context.Context)
	Close() error
	HandlePublishAck(puback *packets.Puback)
}
