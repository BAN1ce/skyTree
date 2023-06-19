package qos

import (
	"bytes"
	"github.com/Pallinder/go-randomdata"
	"github.com/eclipse/paho.golang/packets"
	"io"
	"reflect"
	"testing"
)

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
				topic:    tt.fields.topic,
				writer:   tt.fields.writer,
				listener: tt.fields.listener,
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
	)
	type args struct {
		topic  string
		writer io.Writer
	}
	tests := []struct {
		name string
		args args
		want *QoS0
	}{
		{
			name: "new qos0",
			args: args{
				topic:  topic,
				writer: writer,
			},
			want: &QoS0{
				topic:  topic,
				writer: writer,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			got := NewQoS0(tt.args.topic, writer)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewQoS0() = %v, want %v", got, tt.want)
			}
		})
	}
}
