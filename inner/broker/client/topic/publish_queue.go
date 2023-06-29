package topic

import (
	"container/list"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/eclipse/paho.golang/packets"
	"github.com/google/uuid"
	"time"
)

type PublishQueue struct {
	list   *list.List
	writer PublishWriter
}
type queueElement struct {
	packet    *packets.Publish
	retryKey  string
	messageID string
}

func NewPublishQueue(writer PublishWriter) *PublishQueue {
	return &PublishQueue{
		list:   list.New(),
		writer: writer,
	}
}

func (q *PublishQueue) WritePacket(packet packet.Publish) {
	var (
		retryKey = uuid.NewString()
	)
	q.list.PushBack(&queueElement{
		packet:    packet.Packet,
		retryKey:  retryKey,
		messageID: packet.MessageID,
	})
	err := facade.GetPublishRetry().Create(&retry.Task{
		MaxTimes:     3,
		MaxTime:      60 * time.Second,
		IntervalTime: 10 * time.Second,
		Key:          retryKey,
		Data:         packet,
		Job: func(task *retry.Task) {
			if p, ok := task.Data.(*packets.Publish); ok {
				p.Duplicate = true
				q.writer.WritePacket(p)
			}
		},
		TimeoutJob: func(task *retry.Task) {
			// TODO: close client
			if err := q.writer.Close(); err != nil {
				logger.Logger.Error("close client error: ", err)
			}
		},
	})
	if err != nil {
		logger.Logger.Error("create retry task error: ", err)
	}

}

func (q *PublishQueue) Close() error {
	for e := q.list.Front(); e != nil; e = e.Next() {
		facade.GetPublishRetry().Delete(e.Value.(*queueElement).retryKey)
		q.list.Remove(e)
	}
	return nil
}

func (q *PublishQueue) HandlePublishAck(publishAck *packets.Puback) bool {
	var success bool
	for e := q.list.Front(); e != nil; e = e.Next() {
		if publishAck.PacketID == e.Value.(*queueElement).packet.PacketID {
			logger.Logger.Debug("delete publish retry task: ", e.Value.(*queueElement).retryKey)
			facade.GetPublishRetry().Delete(e.Value.(*queueElement).retryKey)
			q.list.Remove(e)
			success = true
			break
		}
	}
	return success
}

func (q *PublishQueue) GetUnAckMessageID() []string {
	var messageIDs []string
	for e := q.list.Front(); e != nil; e = e.Next() {
		messageIDs = append(messageIDs, e.Value.(*queueElement).messageID)
	}
	return messageIDs
}
