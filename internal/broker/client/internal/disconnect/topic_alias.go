package disconnect

import (
	"errors"
	"fmt"

	"github.com/BAN1ce/skyTree/internal/broker/client/internal/topicalias"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// ForTopicAliasError creates a DISCONNECT packet for Topic Alias violations.
func ForTopicAliasError(err error) *packets.ControlPacket {
	if err == nil {
		return NewServerDisconnect(packets.DisconnectTopicAliasInvalid, "topic alias invalid")
	}
	// MQTT5: alias value out-of-range uses 0x94, but unresolved alias mapping
	// (topic name omitted before mapping exists) is a protocol error 0x82.
	if errors.Is(err, topicalias.ErrTopicAliasNotFound) {
		return NewServerDisconnect(
			packets.DisconnectProtocolError,
			fmt.Sprintf("protocol error: topic alias not found: %s", err.Error()),
		)
	}
	if errors.Is(err, topicalias.ErrTopicAliasInvalid) {
		return NewServerDisconnect(
			packets.DisconnectTopicAliasInvalid,
			fmt.Sprintf("topic alias invalid: %s", err.Error()),
		)
	}
	return NewServerDisconnect(
		packets.DisconnectProtocolError,
		fmt.Sprintf("protocol error: %s", err.Error()),
	)
}
