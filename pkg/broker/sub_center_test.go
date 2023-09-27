package broker

import (
	"testing"
)

func TestInt32ToQoS(t *testing.T) {
	type args struct {
		qos int32
	}
	tests := []struct {
		name string
		args args
		want QoS
	}{
		{
			name: "qos0",
			args: args{
				qos: 0,
			},
			want: 0x00,
		},
		{
			name: "qos1",
			args: args{
				qos: 1,
			},
			want: 0x01,
		},
		{
			name: "qos2",
			args: args{
				qos: 2,
			},
			want: 0x02,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Int32ToQoS(tt.args.qos); got != tt.want {
				t.Errorf("Int32ToQoS() = %v, want %v", got, tt.want)
			}
		})
	}
}
