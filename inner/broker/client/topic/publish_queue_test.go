package topic

import (
	"container/list"
	"github.com/eclipse/paho.golang/packets"
	"reflect"
	"testing"
)

func TestPublishQueue_PushBack(t *testing.T) {
	var (
		pubTask = &PublishTask{
			messageID: "123",
		}
	)
	type fields struct {
		list *list.List
	}
	type args struct {
		task *PublishTask
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "append",
			fields: fields{
				list: list.New(),
			},
			args: args{
				pubTask,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &PublishQueue{
				list: tt.fields.list,
			}
			q.PushBack(tt.args.task)
			if q.Front() != tt.args.task {
				t.Errorf("PushBack() error")
			}
		})
	}
}

func TestPublishQueue_Remove(t *testing.T) {
	var (
		qu    = list.New()
		task1 = &PublishTask{
			messageID: "123",
		}
		task2 = &PublishTask{
			messageID: "123",
		}
		task3 = &PublishTask{
			messageID: "123",
		}
	)
	qu.PushBack(task1)
	qu.PushBack(task2)
	qu.PushBack(task3)

	type fields struct {
		list *list.List
	}
	type args struct {
		task *PublishTask
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "first",
			fields: fields{
				list: qu,
			},
			args: args{
				task1,
			},
		},
		{
			name: "empty",
			fields: fields{
				list: qu,
			},
			args: args{
				nil,
			},
		},
		{
			name: "third",
			fields: fields{
				list: qu,
			},
			args: args{
				task3,
			},
		},
		{
			name: "second",
			fields: fields{
				list: qu,
			},
			args: args{
				task2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &PublishQueue{
				list: tt.fields.list,
			}
			q.Remove(tt.args.task)
		})
	}
	if qu.Len() != 0 {
		t.Errorf("Remove() error")
	}
}

func TestPublishQueue_Range(t *testing.T) {
	var (
		qu = list.New()

		task1 = &PublishTask{
			messageID: "123",
		}
		task2 = &PublishTask{
			messageID: "123",
		}
		task3 = &PublishTask{
			messageID: "123",
		}
		rangeExpectResult []*PublishTask
	)
	qu.PushBack(task1)
	qu.PushBack(task2)
	qu.PushBack(task3)
	type fields struct {
		list   *list.List
		result []*PublishTask
	}
	type args struct {
		f func(task *PublishTask) bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*PublishTask
	}{
		{
			name: "range",
			fields: fields{
				list:   qu,
				result: []*PublishTask{},
			},
			args: args{
				func(task *PublishTask) bool {
					rangeExpectResult = append(rangeExpectResult, task)
					return true
				},
			},
			want: []*PublishTask{
				task1,
				task2,
				task3,
			},
		},
		{
			name: "false",
			fields: fields{
				list:   qu,
				result: []*PublishTask{},
			},
			args: args{
				func(task *PublishTask) bool {
					return false
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &PublishQueue{
				list: tt.fields.list,
			}
			q.Range(tt.args.f)
			if reflect.DeepEqual(tt.fields.result, tt.want) {
				t.Errorf("Range() error")
			}
		})
	}
}

func TestPubTask_SetDuplicate(t1 *testing.T) {
	type fields struct {
		messageID string
		packet    *packets.Publish
		retryKey  string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "",
			fields: fields{
				messageID: "",
				packet: &packets.Publish{
					Duplicate: false,
				},
				retryKey: "",
			},
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &PublishTask{
				messageID: tt.fields.messageID,
				packet:    tt.fields.packet,
				retryKey:  tt.fields.retryKey,
			}
			t.SetDuplicate()
			if !t.packet.Duplicate {
				t1.Errorf("SetDuplicate() error")
			}
		})
	}
}

func TestPubTask_GetPacket(t1 *testing.T) {
	var (
		packet = &packets.Publish{}
	)
	type fields struct {
		messageID string
		packet    *packets.Publish
		retryKey  string
	}
	tests := []struct {
		name   string
		fields fields
		want   *packets.Publish
	}{
		{
			name: "simple",
			fields: fields{
				messageID: "",
				packet:    packet,
				retryKey:  "",
			},
			want: packet,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &PublishTask{
				messageID: tt.fields.messageID,
				packet:    tt.fields.packet,
				retryKey:  tt.fields.retryKey,
			}
			if got := t.GetPacket(); !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("GetPacket() = %v, want %v", got, tt.want)
			}
		})
	}
}
