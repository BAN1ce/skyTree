package retry

import (
	"github.com/Pallinder/go-randomdata"
	"reflect"
	"testing"
	"time"
)

func Test_newDelayTask(t *testing.T) {
	var (
		key   = randomdata.RandStringRunes(10)
		delay = time.Duration(randomdata.Number(10)) * time.Second
		data  = randomdata.RandStringRunes(100)
	)
	type args struct {
		key   string
		delay time.Duration
		data  interface{}
	}
	tests := []struct {
		name string
		args args
		want *DelayTask
	}{
		{
			name: "init",
			args: args{
				key:   key,
				delay: delay,
				data:  data,
			},
			want: &DelayTask{
				key:   key,
				delay: delay,
				data:  data,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newDelayTask(tt.args.key, tt.args.delay, tt.args.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newDelayTask() = %v, want %v", got, tt.want)
			}
		})
	}
}
