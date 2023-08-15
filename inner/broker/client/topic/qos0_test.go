package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic/mock"
	"github.com/golang/mock/gomock"
	"testing"
)

func TestQoS0_afterClose(t1 *testing.T) {
	var (
		ctrl      = gomock.NewController(t1)
		mockEvent = mock.NewMockPublishListener(ctrl)
		topic     = "topic"
		t         = &QoS0{
			topic:           topic,
			publishListener: mockEvent,
		}
		success bool
	)
	mockEvent.EXPECT().DeletePublishEvent(gomock.Eq(topic), gomock.InAnyOrder(t.handler)).Do(func() {
		success = true
	})
	t.afterClose()
	if !success {
		t1.Error("QoS0.afterClose() failed")
	}
}

func TestQoS0_handler(t1 *testing.T) {
	type fields struct {
		ctx             context.Context
		cancel          context.CancelFunc
		topic           string
		writer          PublishWriter
		publishListener PublishListener
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
			name: "topic not string",
			fields: fields{
				ctx:             nil,
				cancel:          nil,
				topic:           "",
				writer:          nil,
				publishListener: nil,
			},
			args: args{
				i: []interface{}{
					1,
				},
			},
		},
		{
			name: "not publish",
			fields: fields{
				ctx:             nil,
				cancel:          nil,
				topic:           "",
				writer:          nil,
				publishListener: nil,
			},
			args: args{
				i: []interface{}{
					"topic",
					1,
				},
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &QoS0{
				ctx:             tt.fields.ctx,
				cancel:          tt.fields.cancel,
				topic:           tt.fields.topic,
				writer:          tt.fields.writer,
				publishListener: tt.fields.publishListener,
			}
			t.handler(tt.args.i...)
		})
	}
}
