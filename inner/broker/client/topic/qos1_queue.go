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
	"time"
)

type PublishQueue struct {
	list   *list.List // publishTask
	writer PublishWriter
}

/**
 * publishTask is the element of the publishing queue.
 * It contains the publishing packet, retry key and message id.
 * The retry key is used to retry the publishing packet.
 * The message id is stored in DB.
 */
type publishTask struct {
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

// WritePacket writes the publishing packet to the publishing queue.
func (q *PublishQueue) WritePacket(packet *packet.PublishMessage) {
	var (
		retryKey = uuid.NewString()
	)
	q.list.PushBack(&publishTask{
		packet:    packet.Packet,
		retryKey:  retryKey,
		messageID: packet.MessageID,
	})
	q.createRetry(retryKey, packet.Packet)
}

func (q *PublishQueue) Close() error {
	for e := q.list.Front(); e != nil; e = e.Next() {
		q.deleteElement(e)
	}
	return nil
}

// HandlePublishAck handles the publishing ack packet.
// match the packet id of the ack packet with the packet id in the queue. packet id means the identifier of the packet.
// If the packet id of the ack packet is in the queue, it will be removed from the queue. and the retry task will be deleted.
// return true if the packet id is in the queue, otherwise return false.
func (q *PublishQueue) HandlePublishAck(publishAck *packets.Puback) bool {
	var success bool
	for e := q.list.Front(); e != nil; e = e.Next() {
		if publishAck.PacketID == e.Value.(*publishTask).packet.PacketID && publishAck.ReasonCode == packets.PubackSuccess {
			logger.Logger.Debug("delete publish retry task: ", zap.String("retryKey", e.Value.(*publishTask).retryKey))
			q.deleteElement(e)
			success = true
			break
		}
	}
	return success
}

// GetUnAckMessageID returns the message id those are not acked.
func (q *PublishQueue) GetUnAckMessageID() []string {
	var messageIDs []string
	for e := q.list.Front(); e != nil; e = e.Next() {
		messageIDs = append(messageIDs, e.Value.(*publishTask).messageID)
	}
	return messageIDs
}

func (q *PublishQueue) deleteElement(e *list.Element) {
	facade.GetPublishRetry().Delete(e.Value.(*publishTask).retryKey)
	q.list.Remove(e)
}

func (q *PublishQueue) createRetry(retryKey string, packet *packets.Publish) {
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
