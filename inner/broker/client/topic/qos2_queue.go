package topic

import (
	"container/list"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/eclipse/paho.golang/packets"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

type QoS2Queue struct {
	list   *list.List // QoS2Task
	writer PublishWriter
}

type QoS2Task struct {
	packet    *packets.Publish
	packetID  uint16
	received  atomic.Bool
	retryKey  string
	messageID string
}

func NewQoS2Queue(writer PublishWriter) *QoS2Queue {
	return &QoS2Queue{
		list:   list.New(),
		writer: writer,
	}
}

func (q *QoS2Queue) WritePacket(message *packet.PublishMessage) {
	var (
		retryKey = uuid.NewString()
		task     = &QoS2Task{
			packet:    message.Packet,
			packetID:  message.Packet.PacketID,
			retryKey:  retryKey,
			messageID: message.MessageID,
		}
	)
	q.list.PushBack(task)
	q.createRetry(retryKey, task)
}

func (q *QoS2Queue) Close() error {
	for e := q.list.Front(); e != nil; e = e.Next() {
		q.deleteElement(e)
	}
	return nil
}

func (q *QoS2Queue) HandlePublishRec(pubrec *packets.Pubrec) bool {
	var success bool
	for e := q.list.Front(); e != nil; e = e.Next() {
		task := e.Value.(*QoS2Task)
		if pubrec.PacketID == task.packetID && pubrec.ReasonCode == packets.PubackSuccess {
			logger.Logger.Debug("delete publish retry task: ", zap.String("retryKey", e.Value.(*QoS2Task).retryKey))
			task.received.Store(true)
			publishRel := packets.NewControlPacket(packets.PUBREL).Content.(*packets.Pubrel)
			publishRel.PacketID = pubrec.PacketID
			q.writer.WritePacket(publishRel)
			break
		}
	}
	return success
}

func (q *QoS2Queue) HandlePublishComp(pubcomp *packets.Pubcomp) {
	for e := q.list.Front(); e != nil; e = e.Next() {
		task := e.Value.(*QoS2Task)
		if !task.received.Load() {
			logger.Logger.Warn("receive pubcomp but not receive pubrec", zap.Uint16("packetID", pubcomp.PacketID))
			return
		}
		if pubcomp.PacketID == task.packetID {
			facade.GetPublishRetry().Delete(task.retryKey)
			q.deleteElement(e)
			break
		}
	}
}

func (q *QoS2Queue) GetUnFinishMessage() []string {
	var messageIDs []string
	for e := q.list.Front(); e != nil; e = e.Next() {
		messageIDs = append(messageIDs, e.Value.(*QoS2Task).messageID)
	}
	return messageIDs
}

func (q *QoS2Queue) deleteElement(e *list.Element) {
	facade.GetPublishRetry().Delete(e.Value.(*QoS2Task).retryKey)
	q.list.Remove(e)
}

func (q *QoS2Queue) createRetry(retryKey string, packet *QoS2Task) {
	err := facade.GetPublishRetry().Create(&retry.Task{
		MaxTimes:     3,
		MaxTime:      60 * time.Second,
		IntervalTime: 10 * time.Second,
		Key:          retryKey,
		Data:         packet,
		Job: func(task *retry.Task) {
			if t, ok := task.Data.(*QoS2Task); ok {
				if t.received.Load() {
					var publishRel = packets.NewControlPacket(packets.PUBREL).Content.(*packets.Pubrel)
					publishRel.PacketID = t.packet.PacketID
					q.writer.WritePacket(publishRel)
				} else {
					q.writer.WritePacket(t.packet)
				}
			}
		},
		TimeoutJob: func(task *retry.Task) {
			// close client
			if err := q.writer.Close(); err != nil {
				logger.Logger.Warn("close client error", zap.Error(err))
			}
		},
	})
	if err != nil {
		logger.Logger.Error("create retry task error: ", zap.Error(err), zap.String("retryKey", retryKey))
	}
}
