package broker

import (
	"reflect"
	"strings"
	"testing"
)

func TestTopicKey(t *testing.T) {
	var (
		topic  = "testTopic"
		result = strings.Builder{}
	)
	result.WriteString(KeyTopicPrefix)
	result.WriteString(topic)
	type args struct {
		topic string
	}
	tests := []struct {
		name string
		args args
		want *strings.Builder
	}{
		{
			name: topic,
			args: args{
				topic: topic,
			},
			want: &result,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TopicKey(tt.args.topic); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TopicKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTopicWillMessageMessageIDKey(t *testing.T) {
	var (
		topic     = "testTopic"
		messageID = "testMessageID"
		result    = strings.Builder{}
	)
	result.WriteString(KeyTopicPrefix)
	result.WriteString(topic)
	result.WriteString(KeyTopicWillMessage)
	result.WriteString("/")
	result.WriteString(messageID)
	type args struct {
		topic     string
		messageID string
	}
	tests := []struct {
		name string
		args args
		want *strings.Builder
	}{
		{
			name: "default",
			args: args{
				topic:     topic,
				messageID: messageID,
			},
			want: &result,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TopicWillMessageMessageIDKey(tt.args.topic, tt.args.messageID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TopicWillMessageMessageIDKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
