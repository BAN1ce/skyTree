package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/broker/client/topic/mock"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/golang/mock/gomock"
	"testing"
)

func TestTopics_DeleteTopic(t1 *testing.T) {
	var (
		ctl    = gomock.NewController(t1)
		topic1 = mock.NewMockTopic(ctl)
		topic2 = mock.NewMockTopic(ctl)
		topic  = map[string]Topic{
			"topic1": topic1,
			"topic2": topic2,
		}
	)
	defer ctl.Finish()
	topic1.EXPECT().Close().Return(nil).Times(1)
	topic2.EXPECT().Close().Return(nil).Times(1)
	type fields struct {
		ctx        context.Context
		topic      map[string]Topic
		session    pkg.SessionTopic
		store      pkg.ClientMessageStore
		writer     PublishWriter
		windowSize int
	}
	type args struct {
		topicName string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "topic1",
			fields: fields{
				topic: topic,
			},
			args: args{
				topicName: "topic1",
			},
		},
		{
			name: "topic2",
			fields: fields{
				topic: topic,
			},
			args: args{
				topicName: "topic2",
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Topics{
				ctx:        tt.fields.ctx,
				topic:      tt.fields.topic,
				session:    tt.fields.session,
				store:      tt.fields.store,
				writer:     tt.fields.writer,
				windowSize: tt.fields.windowSize,
			}
			t.DeleteTopic(tt.args.topicName)
			if _, ok := t.topic[tt.args.topicName]; ok {
				t1.Errorf("topic %s is not deleted", tt.args.topicName)
			}
		})
	}

}

func TestTopics_Close(t1 *testing.T) {
	var (
		ctl1   = gomock.NewController(t1)
		ctl2   = gomock.NewController(t1)
		topic1 = mock.NewMockTopic(ctl1)
		topic2 = mock.NewMockTopic(ctl2)
		topic  = map[string]Topic{
			"topic1": topic1,
			"topic2": topic2,
		}
	)
	defer func() {
		ctl1.Finish()
		ctl2.Finish()
	}()
	topic1.EXPECT().Close().Return(nil).Times(1)
	topic2.EXPECT().Close().Return(nil).Times(1)

	type fields struct {
		ctx        context.Context
		topic      map[string]Topic
		session    pkg.SessionTopic
		store      pkg.ClientMessageStore
		writer     PublishWriter
		windowSize int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "close all",
			fields: fields{
				ctx:        nil,
				topic:      topic,
				session:    nil,
				store:      nil,
				writer:     nil,
				windowSize: 0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Topics{
				ctx:        tt.fields.ctx,
				topic:      tt.fields.topic,
				session:    tt.fields.session,
				store:      tt.fields.store,
				writer:     tt.fields.writer,
				windowSize: tt.fields.windowSize,
			}
			if err := t.Close(); (err != nil) != tt.wantErr {
				t1.Errorf("Close() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(t.topic) != 0 {
				t1.Errorf("Close() topic is not empty")
			}
		})
	}
}
