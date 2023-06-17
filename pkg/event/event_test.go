package event

import (
	"github.com/kataras/go-events"
	"testing"
)

func TestWithEventPrefix(t *testing.T) {
	type args struct {
		name Name
		s    string
	}
	tests := []struct {
		name string
		args args
		want events.EventName
	}{
		{
			name: "test",
			args: args{
				name: PublishQoS0Prefix,
				s:    "test",
			},
			want: "event_publish_qos0_test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithEventPrefix(tt.args.name, tt.args.s); got != tt.want {
				t.Errorf("WithEventPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
