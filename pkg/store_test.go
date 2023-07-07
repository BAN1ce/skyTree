package pkg

import (
	"github.com/eclipse/paho.golang/packets"
	"reflect"
	"testing"
)

func TestDecode(t *testing.T) {
	type args struct {
		payload    []byte
		qos        byte
		topic      string
		packetID   uint16
		retain     bool
		duplicate  bool
		properties *packets.Properties
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "no properties",
			args: args{
				payload:    []byte("test"),
				qos:        0,
				topic:      "test",
				packetID:   0x12,
				retain:     false,
				duplicate:  false,
				properties: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				publish, _ = packets.NewControlPacket(packets.PUBLISH).Content.(*packets.Publish)
			)
			publish.QoS = tt.args.qos
			publish.Topic = "test"
			publish.Payload = []byte("test")
			publish.Retain = tt.args.retain
			publish.Duplicate = tt.args.duplicate

			rawData, err := Encode(publish)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}
			if newPublish, err := Decode(rawData); err != nil {
				t.Error("decode error", err)
			} else {
				if !reflect.DeepEqual(newPublish, publish) {
					t.Errorf("decode error, got %v, want %v", newPublish, publish)
				}
			}

		})
	}
}
