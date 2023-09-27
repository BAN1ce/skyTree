package qos2

import (
	"container/list"
	"fmt"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/eclipse/paho.golang/packets"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

type Queue struct {
	list   *list.List // QoS2Task
	writer broker.PublishWriter
}

type QoS2Task struct {
	packet    *packets.Publish
	packetID  uint16
	received  atomic.Bool
	retryKey  string
	messageID string
}

func NewQoS2Queue(writer broker.PublishWriter) *Queue {
	return &Queue{
		list:   list.New(),
		writer: writer,
	}
}

func (q *Queue) WritePacket(message *packet.PublishMessage) {
	var (
		retryKey = uuid.NewString()
		task     = &QoS2Task{
			packet:    message.PublishPacket,
			packetID:  message.PublishPacket.PacketID,
			retryKey:  retryKey,
			messageID: message.MessageID,
		}
	)
	if message.PubReceived {
		task.received.Store(true)
	}
	q.list.PushBack(task)
	q.createRetry(retryKey, task)
}

func (q *Queue) Close() error {
	for e := q.list.Front(); e != nil; e = e.Next() {
		q.deleteElement(e)
	}
	return nil
}

func (q *Queue) HandlePublishRec(pubrec *packets.Pubrec) bool {
	var success bool
	for e := q.list.Front(); e != nil; e = e.Next() {
		task := e.Value.(*QoS2Task)
		if pubrec.PacketID == task.packetID && pubrec.ReasonCode == packets.PubackSuccess {
			logger.Logger.Debug("delete publish rec retry task: ", zap.Uint16("packetID", pubrec.PacketID), zap.String("retryKey", e.Value.(*QoS2Task).retryKey))
			task.received.Store(true)
			publishRel := packets.NewControlPacket(packets.PUBREL).Content.(*packets.Pubrel)
			publishRel.PacketID = pubrec.PacketID
			q.writer.WritePacket(publishRel)
			break
		}
	}
	return success
}

func (q *Queue) HandlePublishComp(pubcomp *packets.Pubcomp) {
	for e := q.list.Front(); e != nil; e = e.Next() {
		task := e.Value.(*QoS2Task)
		if !task.received.Load() {
			logger.Logger.Warn("receive pubcomp but not receive pubrec", zap.Uint16("packetID", pubcomp.PacketID))
			return
		}
		if pubcomp.PacketID == task.packetID {
			facade.DeletePublishRetryKey(task.retryKey)
			q.deleteElement(e)
			break
		}
	}
}

func (q *Queue) deleteElement(e *list.Element) {
	facade.GetPublishRetry().Delete(e.Value.(*QoS2Task).retryKey)
	q.list.Remove(e)
}

func (q *Queue) createRetry(retryKey string, packet *QoS2Task) {
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

func (q *Queue) getUnFinishPacket() []broker.UnFinishedMessage {
	var unFinishMessage []broker.UnFinishedMessage
	for e := q.list.Front(); e != nil; e = e.Next() {
		task := e.Value.(*QoS2Task)
		unFinishMessage = append(unFinishMessage, broker.UnFinishedMessage{
			MessageID:   task.messageID,
			PacketID:    fmt.Sprintf("%d", task.packetID),
			PubReceived: task.received.Load(),
		})
	}
	return unFinishMessage
}
