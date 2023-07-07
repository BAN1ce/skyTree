package topic

import (
	"container/list"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"reflect"
	"strconv"
	"testing"
)

func TestPublishQueue_GetUnAckMessageID(t *testing.T) {
	var (
		testList       = list.New()
		unAckMessageID = []string{
			"1",
			"2",
			"3",
		}
	)
	for _, id := range unAckMessageID {
		testList.PushBack(&publishTask{
			messageID: id,
		})
	}
	type fields struct {
		list   *list.List
		writer PublishWriter
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "get unAckMessageID",
			fields: fields{
				list:   testList,
				writer: nil,
			},
			want: unAckMessageID,
		},
		{
			name: "empty list",
			fields: fields{
				list: list.New(),
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &PublishQueue{
				list:   tt.fields.list,
				writer: tt.fields.writer,
			}
			if got := q.GetUnAckMessageID(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetUnAckMessageID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mockPublishQueue() *PublishQueue {
	var (
		publishQueue = NewPublishQueue(newMockPublishWriter())
		publish      []*packets.Publish
	)
	for i := 0; i < 10; i++ {
		publish = append(publish, &packets.Publish{
			PacketID: uint16(i),
			Topic:    "test",
			Payload:  []byte("test"),
		})
	}
	for i, p := range publish {
		publishQueue.WritePacket(&packet.PublishMessage{
			MessageID: strconv.Itoa(i),
			Packet:    p,
		})
	}
	return publishQueue
}

func TestPublishQueue_HandlePublishAck(t *testing.T) {
	var (
		publishQueue = mockPublishQueue()
		pubAck       = []*packets.Puback{}
	)
	for i := 0; i < 10; i++ {
		pubAck = append(pubAck, &packets.Puback{
			PacketID:   uint16(i),
			ReasonCode: packets.PubackSuccess,
		})
	}

	for _, ack := range pubAck {
		publishQueue.HandlePublishAck(ack)
	}
	if publishQueue.list.Len() != 0 {
		t.Errorf("publishQueue list length = %d, want %d", publishQueue.list.Len(), 0)
	}
}
