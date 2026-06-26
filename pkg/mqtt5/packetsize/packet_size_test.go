package packetsize

import (
	"errors"
	"strings"
	"testing"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestCheckPacketSizeWithMax(t *testing.T) {
	tests := []struct {
		name        string
		packet      *packets.ControlPacket
		maxSize     int
		expectError bool
	}{
		{
			name:        "small publish control packet",
			packet:      createPublishControlPacket("test/topic", []byte("small message"), 0),
			maxSize:     1024,
			expectError: false,
		},
		{
			name:        "large publish control packet",
			packet:      createPublishControlPacket("test/topic", make([]byte, 4096), 0),
			maxSize:     256,
			expectError: true,
		},
		{
			name:        "small connect control packet",
			packet:      createConnectControlPacket("test_client"),
			maxSize:     1024,
			expectError: false,
		},
		{
			name:        "large connect control packet",
			packet:      createConnectControlPacket(strings.Repeat("a", 4096)),
			maxSize:     256,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPacketSizeWithMax(tt.packet, tt.maxSize)
			if tt.expectError && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectError && !errors.Is(err, ErrPacketOversize) {
				t.Fatalf("expected ErrPacketOversize, got %v", err)
			}
		})
	}
}

func TestCheckPublishPacketSizeWithMax(t *testing.T) {
	tests := []struct {
		name        string
		publish     *packets.Publish
		maxSize     int
		expectError bool
	}{
		{
			name:        "small publish",
			publish:     createSmallPublish(),
			maxSize:     1024,
			expectError: false,
		},
		{
			name:        "large publish",
			publish:     createLargePublish(),
			maxSize:     256,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPublishPacketSizeWithMax(tt.publish, tt.maxSize)
			if tt.expectError && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectError && !errors.Is(err, ErrPacketOversize) {
				t.Fatalf("expected ErrPacketOversize, got %v", err)
			}
		})
	}
}

func TestCheckConnectPacketSizeWithMax(t *testing.T) {
	tests := []struct {
		name        string
		connect     *packets.Connect
		maxSize     int
		expectError bool
	}{
		{
			name:        "small connect",
			connect:     createSmallConnect(),
			maxSize:     1024,
			expectError: false,
		},
		{
			name:        "large connect",
			connect:     createLargeConnect(),
			maxSize:     256,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckConnectPacketSizeWithMax(tt.connect, tt.maxSize)
			if tt.expectError && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectError && !errors.Is(err, ErrPacketOversize) {
				t.Fatalf("expected ErrPacketOversize, got %v", err)
			}
		})
	}
}

func TestCheckPublishPacketSizeWithMaxUsesFullWireEncoding(t *testing.T) {
	topicAlias := uint16(7)
	messageExpiry := uint32(60)
	payloadFormat := byte(1)
	publish := &packets.Publish{
		Topic:    "sensors/temperature",
		Payload:  []byte(`{"c":21}`),
		QoS:      1,
		PacketID: 42,
		Properties: &packets.PublishProperties{
			TopicAlias:    &topicAlias,
			MessageExpiry: &messageExpiry,
			PayloadFormat: &payloadFormat,
			ContentType:   "application/json",
			User:          []packets.User{{Key: "source", Value: "unit-test"}},
		},
	}
	cp := packets.NewControlPacket(packets.PUBLISH)
	cp.Content = publish
	packetSize, err := GetPacketSize(cp)
	if err != nil {
		t.Fatalf("GetPacketSize: %v", err)
	}
	if err := CheckPublishPacketSizeWithMax(publish, packetSize); err != nil {
		t.Fatalf("expected publish size %d to be accepted, got %v", packetSize, err)
	}
	if err := CheckPublishPacketSizeWithMax(publish, packetSize-1); !errors.Is(err, ErrPacketOversize) {
		t.Fatalf("expected ErrPacketOversize when max=%d, got %v", packetSize-1, err)
	}
}

func createSmallPublish() *packets.Publish {
	return &packets.Publish{
		Topic:   "test/topic",
		Payload: []byte("small message"),
		QoS:     0,
	}
}

func createLargePublish() *packets.Publish {
	return &packets.Publish{
		Topic:   "test/topic",
		Payload: make([]byte, 4096),
		QoS:     0,
	}
}

func createSmallConnect() *packets.Connect {
	return &packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 5,
		ClientID:        "test_client",
		CleanStart:      true,
		KeepAlive:       60,
	}
}

func createLargeConnect() *packets.Connect {
	return &packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 5,
		ClientID:        strings.Repeat("a", 4096),
		CleanStart:      true,
		KeepAlive:       60,
	}
}

func createPublishControlPacket(topic string, payload []byte, qos byte) *packets.ControlPacket {
	cp := packets.NewControlPacket(packets.PUBLISH)
	if cp == nil {
		return nil
	}
	pub, _ := cp.Content.(*packets.Publish)
	if pub != nil {
		pub.Topic = topic
		pub.Payload = payload
		pub.QoS = qos
	}
	return cp
}

func createConnectControlPacket(clientID string) *packets.ControlPacket {
	cp := packets.NewControlPacket(packets.CONNECT)
	if cp == nil {
		return nil
	}
	conn, _ := cp.Content.(*packets.Connect)
	if conn != nil {
		conn.ProtocolName = "MQTT"
		conn.ProtocolVersion = 5
		conn.ClientID = clientID
		conn.CleanStart = true
		conn.KeepAlive = 60
	}
	return cp
}
