package qos

import (
	"bytes"
	"context"
	"github.com/Pallinder/go-randomdata"
	"github.com/eclipse/paho.golang/packets"
	"io"
	"reflect"
	"testing"
	"time"
)

type mockTopicPublishListener struct {
	event map[string]func(i ...interface{})
}

func newMockTopicPublishListener() *mockTopicPublishListener {
	return &mockTopicPublishListener{
		event: make(map[string]func(i ...interface{})),
	}
}

func (m *mockTopicPublishListener) CreatePublishEvent(topic string, handler func(...interface{})) {
	m.event[topic] = handler
}

func (m *mockTopicPublishListener) DeletePublishEvent(topic string, handler func(i ...interface{})) {
	delete(m.event, topic)
}

func TestQoS0_publish(t1 *testing.T) {
	var (
		topic   = "test"
		publish = &packets.Publish{
			Payload:    []byte("test"),
			Topic:      topic,
			Properties: nil,
			PacketID:   123,
			QoS:        0,
			Duplicate:  false,
			Retain:     false,
		}
		expect = bytes.NewBuffer(nil)
		writer = bytes.NewBuffer(nil)
	)
	if _, err := publish.WriteTo(expect); err != nil {
		t1.Error(err)
	}
	type fields struct {
		topic    string
		writer   io.Writer
		listener func(i ...interface{})
	}
	type args struct {
		topic   string
		publish *packets.Publish
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "write",
			fields: fields{
				topic:    topic,
				writer:   writer,
				listener: nil,
			},
			args: args{
				topic:   topic,
				publish: publish,
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &QoS0{
				topic:  tt.fields.topic,
				writer: tt.fields.writer,
			}
			t.publish(tt.args.topic, tt.args.publish)
			if !bytes.Equal(writer.Bytes(), expect.Bytes()) {
				t1.Errorf("QoS0.publish() = %v, want %v", writer.Bytes(), expect.Bytes())
			} else {
				t1.Log("QoS0.publish() = ", writer.Bytes(), "want", expect.Bytes())
			}
		})
	}
}

func TestNewQoS0(t *testing.T) {
	var (
		topic  = randomdata.SillyName()
		writer = bytes.NewBuffer(nil)
		mock   = newMockTopicPublishListener()
	)
	type args struct {
		topic           string
		writer          io.Writer
		publishListener TopicPublishListener
	}
	tests := []struct {
		name string
		args args
		want *QoS0
	}{
		{
			name: "new qos0",
			args: args{
				topic:           topic,
				writer:          writer,
				publishListener: mock,
			},
			want: &QoS0{
				topic:           topic,
				writer:          writer,
				publishListener: mock,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewQoS0(tt.args.topic, tt.args.writer, tt.args.publishListener)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewQoS0() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQoS0_Start(t1 *testing.T) {
	var (
		topic  = randomdata.SillyName()
		writer = bytes.NewBuffer(nil)
		mock   = newMockTopicPublishListener()
	)
	type fields struct {
		topic           string
		writer          io.Writer
		publishListener TopicPublishListener
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "publish one",
			fields: fields{
				topic:           topic,
				writer:          writer,
				publishListener: mock,
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			t := &QoS0{
				topic:           tt.fields.topic,
				writer:          tt.fields.writer,
				publishListener: tt.fields.publishListener,
			}
			cancel()
			t.Start(ctx)
			if len(mock.event) != 0 {
				t1.Error("QoS0.Start() = ", len(mock.event), "want", 0)
			}
		})
	}
}

func TestQoS0_handler(t1 *testing.T) {
	var (
		topic  = randomdata.SillyName()
		writer = bytes.NewBuffer(nil)
		mock   = newMockTopicPublishListener()
		packet = &packets.Publish{
			Payload:    []byte(randomdata.RandStringRunes(50)),
			Topic:      topic,
			Properties: nil,
			PacketID:   123,
		}
		packetBuffer = bytes.NewBuffer(nil)
	)
	_, _ = packet.WriteTo(packetBuffer)
	type fields struct {
		topic           string
		writer          io.Writer
		publishListener TopicPublishListener
	}
	type args struct {
		i []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "write publish packet",
			fields: fields{
				topic:           topic,
				writer:          writer,
				publishListener: mock,
			},
			args: args{
				i: []interface{}{topic, packet},
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &QoS0{
				topic:           tt.fields.topic,
				writer:          tt.fields.writer,
				publishListener: tt.fields.publishListener,
			}
			t.handler(tt.args.i...)
			if !bytes.Equal(writer.Bytes(), packetBuffer.Bytes()) && len(writer.Bytes()) != 0 {
				t1.Error("QoS0.handler() = ", writer.Bytes(), "want", packetBuffer.Bytes())
			}
		})
	}
}
