package client

import "testing"

func TestClientNextPacketIDSkipsOutgoingInflightIDs(t *testing.T) {
	c := NewClient(nil)
	c.packetIDFactory.(*PacketIDFactory).SetID(1)
	c.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID: 2,
		QoS:      1,
		State:    outInflightWaitingPubAck,
	})

	if got := c.NextPacketID(); got != 3 {
		t.Fatalf("expected next packet id to skip in-flight 2 and return 3, got %d", got)
	}
}
