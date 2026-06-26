package disconnect

import (
	"fmt"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func ForReceiveMaximumExceeded(inflight, maximum int) *packets.ControlPacket {
	return NewServerDisconnect(
		packets.DisconnectReceiveMaximumExceeded,
		fmt.Sprintf("receive maximum exceeded: inflight %d >= maximum %d", inflight, maximum),
	)
}
