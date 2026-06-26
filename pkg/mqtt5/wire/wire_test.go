package wire_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
)

// TestDecodeRejectsInvalidFixedHeaderFlags 覆盖固定报头标志位非法时的拒绝路径。
func TestDecodeRejectsInvalidFixedHeaderFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "connect with flags",
			raw:  []byte{0x11, 0x00},
		},
		{
			name: "pubrel without required flags",
			raw:  []byte{0x60, 0x02, 0x00, 0x01},
		},
		{
			name: "publish with qos 3",
			raw:  []byte{0x36, 0x00},
		},
		{
			name: "publish qos0 with dup flag",
			raw: appendFixedHeader(mqtt5.PUBLISH, 0x08, []byte{
				0x00, 0x03, 'a', '/', 'b', // topic
				0x00, // property length
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := wire.Decode(bytes.NewReader(tt.raw), wire.DecodeOptions{})
			if err == nil {
				t.Fatal("Decode returned nil error")
			}

			var wireErr *wire.WireError
			if !errors.As(err, &wireErr) {
				t.Fatalf("Decode error = %T, want *wire.WireError", err)
			}
			if wireErr.Kind != wire.ErrMalformedPacket {
				t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
			}
		})
	}
}

// TestEncodeRejectsQoS0PublishWithDup 确认编码侧拒绝 QoS 0 且 DUP=1 的 PUBLISH。
func TestEncodeRejectsQoS0PublishWithDup(t *testing.T) {
	t.Parallel()

	packet := &mqtt5.ControlPacket{
		Content: &mqtt5.Publish{
			Topic:     "a/b",
			Payload:   []byte("hello"),
			QoS:       0,
			Duplicate: true,
		},
	}

	_, err := wire.Encode(packet, wire.EncodeOptions{})
	if err == nil {
		t.Fatal("Encode returned nil error")
	}

	var wireErr *wire.WireError
	if !errors.As(err, &wireErr) {
		t.Fatalf("Encode error = %T, want *wire.WireError", err)
	}
	if wireErr.Kind != wire.ErrMalformedPacket {
		t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
	}
}

// TestDecodeRejectsConnectPasswordWithoutUsername 覆盖 CONNECT PasswordFlag=1 且 UsernameFlag=0 的非法组合。
func TestDecodeRejectsConnectPasswordWithoutUsername(t *testing.T) {
	t.Parallel()

	raw := appendFixedHeader(mqtt5.CONNECT, 0x00, []byte{
		0x00, 0x04, 'M', 'Q', 'T', 'T',
		0x05,
		0x40, // PasswordFlag=1, UsernameFlag=0
	})

	_, err := wire.Decode(bytes.NewReader(raw), wire.DecodeOptions{})
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	var wireErr *wire.WireError
	if !errors.As(err, &wireErr) {
		t.Fatalf("Decode error = %T, want *wire.WireError", err)
	}
	if wireErr.Kind != wire.ErrMalformedPacket {
		t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
	}
}

// TestEncodeRejectsConnectPasswordWithoutUsername 确认编码侧同样拒绝 PasswordFlag=1 且 UsernameFlag=0。
func TestEncodeRejectsConnectPasswordWithoutUsername(t *testing.T) {
	t.Parallel()

	packet := &mqtt5.ControlPacket{
		Content: &mqtt5.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "cid",
			PasswordFlag:    true,
			Password:        []byte("secret"),
		},
	}

	_, err := wire.Encode(packet, wire.EncodeOptions{})
	if err == nil {
		t.Fatal("Encode returned nil error")
	}

	var wireErr *wire.WireError
	if !errors.As(err, &wireErr) {
		t.Fatalf("Encode error = %T, want *wire.WireError", err)
	}
	if wireErr.Kind != wire.ErrMalformedPacket {
		t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
	}
}

func TestEncodeRejectsConnectWillQoSOutOfRange(t *testing.T) {
	t.Parallel()

	packet := &mqtt5.ControlPacket{
		Content: &mqtt5.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "cid",
			WillFlag:        true,
			WillQOS:         4,
			WillTopic:       "a/b",
			WillMessage:     []byte("bye"),
		},
	}

	_, err := wire.Encode(packet, wire.EncodeOptions{})
	if err == nil {
		t.Fatal("Encode returned nil error")
	}

	var wireErr *wire.WireError
	if !errors.As(err, &wireErr) {
		t.Fatalf("Encode error = %T, want *wire.WireError", err)
	}
	if wireErr.Kind != wire.ErrMalformedPacket {
		t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
	}
}

func TestEncodeRejectsSubscribeOptionsOutOfRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sub  mqtt5.SubOptions
	}{
		{
			name: "qos out of range",
			sub: mqtt5.SubOptions{
				Topic: "a/b",
				QoS:   4,
			},
		},
		{
			name: "retain handling out of range",
			sub: mqtt5.SubOptions{
				Topic:          "a/b",
				QoS:            1,
				RetainHandling: 4,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			packet := &mqtt5.ControlPacket{
				Content: &mqtt5.Subscribe{
					PacketID:      1,
					Subscriptions: []mqtt5.SubOptions{tt.sub},
				},
			}
			_, err := wire.Encode(packet, wire.EncodeOptions{})
			if err == nil {
				t.Fatal("Encode returned nil error")
			}

			var wireErr *wire.WireError
			if !errors.As(err, &wireErr) {
				t.Fatalf("Encode error = %T, want *wire.WireError", err)
			}
			if wireErr.Kind != wire.ErrMalformedPacket {
				t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
			}
		})
	}
}

// TestEncodeRejectsStringsLongerThanTwoByteLength 确认超出 MQTT 二字节长度前缀的字符串会被拒绝。
func TestEncodeRejectsStringsLongerThanTwoByteLength(t *testing.T) {
	t.Parallel()

	longTopic := bytes.Repeat([]byte("a"), 65536)
	packet := &mqtt5.ControlPacket{
		Content: &mqtt5.Publish{
			Topic:   string(longTopic),
			Payload: []byte("payload"),
		},
	}

	_, err := wire.Encode(packet, wire.EncodeOptions{})
	if err == nil {
		t.Fatal("Encode returned nil error")
	}

	var wireErr *wire.WireError
	if !errors.As(err, &wireErr) {
		t.Fatalf("Encode error = %T, want *wire.WireError", err)
	}
	if wireErr.Kind != wire.ErrMalformedPacket {
		t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
	}
}

// TestPropertiesRejectDisallowedAndDuplicateSingletons 校验属性适用报文和单例属性重复限制。
func TestPropertiesRejectDisallowedAndDuplicateSingletons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "will delay is not a connect property",
			raw: appendFixedHeader(mqtt5.CONNECT, 0x00, []byte{
				0x00, 0x04, 'M', 'Q', 'T', 'T',
				0x05,
				0x00,
				0x00, 0x3c,
				0x05,
				mqtt5.PropWillDelayInterval, 0x00, 0x00, 0x00, 0x0a,
				0x00, 0x00,
			}),
		},
		{
			name: "duplicate receive maximum",
			raw: appendFixedHeader(mqtt5.CONNECT, 0x00, []byte{
				0x00, 0x04, 'M', 'Q', 'T', 'T',
				0x05,
				0x00,
				0x00, 0x3c,
				0x06,
				mqtt5.PropReceiveMaximum, 0x00, 0x0a,
				mqtt5.PropReceiveMaximum, 0x00, 0x14,
				0x00, 0x00,
			}),
		},
		{
			name: "duplicate subscription identifier in subscribe properties",
			raw: appendFixedHeader(mqtt5.SUBSCRIBE, 0x02, []byte{
				0x00, 0x01, // packet id
				0x04, // property length
				mqtt5.PropSubscriptionIdentifier, 0x01,
				mqtt5.PropSubscriptionIdentifier, 0x02,
				0x00, 0x01, 'a', // topic filter
				0x00, // sub options
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := wire.Decode(bytes.NewReader(tt.raw), wire.DecodeOptions{})
			if err == nil {
				t.Fatal("Decode returned nil error")
			}

			var wireErr *wire.WireError
			if !errors.As(err, &wireErr) {
				t.Fatalf("Decode error = %T, want *wire.WireError", err)
			}
			if wireErr.Kind != wire.ErrMalformedPacket {
				t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
			}
		})
	}
}

// TestDecodeRejectsEmptyReasonCodesForSubackAndUnsuback 覆盖 SUBACK/UNSUBACK 至少包含一个原因码的约束。
func TestDecodeRejectsEmptyReasonCodesForSubackAndUnsuback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "suback without reasons",
			raw: appendFixedHeader(mqtt5.SUBACK, 0x00, []byte{
				0x00, 0x01, // packet id
				0x00, // property length
			}),
		},
		{
			name: "unsuback without reasons",
			raw: appendFixedHeader(mqtt5.UNSUBACK, 0x00, []byte{
				0x00, 0x01, // packet id
				0x00, // property length
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := wire.Decode(bytes.NewReader(tt.raw), wire.DecodeOptions{})
			if err == nil {
				t.Fatal("Decode returned nil error")
			}
			var wireErr *wire.WireError
			if !errors.As(err, &wireErr) {
				t.Fatalf("Decode error = %T, want *wire.WireError", err)
			}
			if wireErr.Kind != wire.ErrMalformedPacket {
				t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
			}
		})
	}
}

// TestEncodeRejectsEmptyReasonCodesForSubackAndUnsuback 确认编码侧也拒绝空原因码列表。
func TestEncodeRejectsEmptyReasonCodesForSubackAndUnsuback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		packet *mqtt5.ControlPacket
	}{
		{
			name: "suback without reasons",
			packet: &mqtt5.ControlPacket{
				Content: &mqtt5.Suback{
					PacketID: 1,
					Reasons:  nil,
				},
			},
		},
		{
			name: "unsuback without reasons",
			packet: &mqtt5.ControlPacket{
				Content: &mqtt5.Unsuback{
					PacketID: 1,
					Reasons:  nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := wire.Encode(tt.packet, wire.EncodeOptions{})
			if err == nil {
				t.Fatal("Encode returned nil error")
			}
			var wireErr *wire.WireError
			if !errors.As(err, &wireErr) {
				t.Fatalf("Encode error = %T, want *wire.WireError", err)
			}
			if wireErr.Kind != wire.ErrMalformedPacket {
				t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
			}
		})
	}
}

func TestDecodeRejectsSharedSubscriptionGroupContainingWildcard(t *testing.T) {
	t.Parallel()

	raw := appendFixedHeader(mqtt5.SUBSCRIBE, 0x02, []byte{
		0x00, 0x01, // packet id
		0x00, // property length
		0x00, 0x0B, '$', 's', 'h', 'a', 'r', 'e', '/', 'g', '+', '/', 'a',
		0x00, // sub options
	})

	_, err := wire.Decode(bytes.NewReader(raw), wire.DecodeOptions{})
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	var wireErr *wire.WireError
	if !errors.As(err, &wireErr) {
		t.Fatalf("Decode error = %T, want *wire.WireError", err)
	}
	if wireErr.Kind != wire.ErrMalformedPacket {
		t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
	}
}

// TestDecodeRejectsNonMinimalVariableByteInteger validates MQTT5 minimal VBI encoding requirements.
func TestDecodeRejectsNonMinimalVariableByteInteger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "remaining length overlong encoding",
			raw:  []byte{0xC0, 0x80, 0x00}, // PINGREQ with Remaining Length 0 encoded in 2 bytes
		},
		{
			name: "subscription identifier vbi overlong encoding",
			raw: appendFixedHeader(mqtt5.SUBSCRIBE, 0x02, []byte{
				0x00, 0x01, // packet id
				0x03,                                         // property length
				mqtt5.PropSubscriptionIdentifier, 0x81, 0x00, // value=1, overlong VBI
				0x00, 0x01, 'a', // topic filter
				0x00, // sub options
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := wire.Decode(bytes.NewReader(tt.raw), wire.DecodeOptions{})
			if err == nil {
				t.Fatal("Decode returned nil error")
			}

			var wireErr *wire.WireError
			if !errors.As(err, &wireErr) {
				t.Fatalf("Decode error = %T, want *wire.WireError", err)
			}
			if wireErr.Kind != wire.ErrMalformedPacket {
				t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrMalformedPacket)
			}
		})
	}
}

// TestEncodeDecodeRoundTrip 确认 PUBLISH 编码、解码、再次编码后字节保持稳定。
func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	topicAlias := uint16(7)
	contentType := "application/json"
	packet := &mqtt5.ControlPacket{
		Content: &mqtt5.Publish{
			Topic:    "sensors/temperature",
			PacketID: 42,
			QoS:      1,
			Payload:  []byte(`{"c":21}`),
			Properties: &mqtt5.PublishProperties{
				ContentType: contentType,
				TopicAlias:  &topicAlias,
				User: []mqtt5.User{
					{Key: "source", Value: "unit-test"},
				},
			},
		},
	}

	encoded, err := wire.Encode(packet, wire.EncodeOptions{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := wire.Decode(bytes.NewReader(encoded), wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	publish, ok := decoded.Content.(*mqtt5.Publish)
	if !ok {
		t.Fatalf("decoded content = %T, want *mqtt5.Publish", decoded.Content)
	}
	if decoded.Type != mqtt5.PUBLISH {
		t.Fatalf("decoded Type = %d, want %d", decoded.Type, mqtt5.PUBLISH)
	}
	if publish.Topic != "sensors/temperature" {
		t.Fatalf("Topic = %q", publish.Topic)
	}
	if publish.PacketID != 42 {
		t.Fatalf("PacketID = %d", publish.PacketID)
	}
	if publish.QoS != 1 {
		t.Fatalf("QoS = %d", publish.QoS)
	}
	if string(publish.Payload) != `{"c":21}` {
		t.Fatalf("Payload = %q", publish.Payload)
	}
	if publish.Properties == nil || publish.Properties.ContentType != contentType {
		t.Fatalf("ContentType = %#v", publish.Properties)
	}
	if publish.Properties.TopicAlias == nil || *publish.Properties.TopicAlias != topicAlias {
		t.Fatalf("TopicAlias = %#v", publish.Properties.TopicAlias)
	}
	if got := publish.Properties.User; len(got) != 1 || got[0].Key != "source" || got[0].Value != "unit-test" {
		t.Fatalf("User properties = %#v", got)
	}

	reencoded, err := wire.Encode(decoded, wire.EncodeOptions{})
	if err != nil {
		t.Fatalf("re-Encode: %v", err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Fatalf("encoded bytes are not stable:\nfirst:  %x\nsecond: %x", encoded, reencoded)
	}
}

// TestDecodeHonorsMaxPacketSizeBeforeReadingBody 确认超限报文在读取报文体前就被拒绝。
func TestDecodeHonorsMaxPacketSizeBeforeReadingBody(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte{'x'}, 8)
	raw := appendFixedHeader(mqtt5.PUBLISH, 0x00, body)
	reader := &trackingReader{data: raw}

	_, err := wire.Decode(reader, wire.DecodeOptions{MaxPacketSize: 4})
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	var wireErr *wire.WireError
	if !errors.As(err, &wireErr) {
		t.Fatalf("Decode error = %T, want *wire.WireError", err)
	}
	if wireErr.Kind != wire.ErrPacketTooLarge {
		t.Fatalf("WireError.Kind = %v, want %v", wireErr.Kind, wire.ErrPacketTooLarge)
	}
	if reader.reads > 2 {
		t.Fatalf("reader read count = %d, want fixed header only", reader.reads)
	}
}

// trackingReader 逐字节返回数据，用于断言解码器不会在超限后继续读 body。
type trackingReader struct {
	data  []byte
	reads int
}

// Read 模拟每次读取只返回一个字节的网络 reader。
func (r *trackingReader) Read(p []byte) (int, error) {
	r.reads++
	if len(r.data) == 0 {
		return 0, errors.New("unexpected body read")
	}
	n := copy(p, r.data[:1])
	r.data = r.data[n:]
	return n, nil
}

// appendFixedHeader 为测试报文拼接固定报头和剩余长度字段。
func appendFixedHeader(packetType mqtt5.PacketType, flags byte, body []byte) []byte {
	out := []byte{byte(packetType)<<4 | flags}
	out = append(out, encodeRemainingLengthForTest(len(body))...)
	out = append(out, body...)
	return out
}

// encodeRemainingLengthForTest 使用测试内实现生成 MQTT 剩余长度字段。
func encodeRemainingLengthForTest(length int) []byte {
	var out []byte
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		out = append(out, digit)
		if length == 0 {
			return out
		}
	}
}
