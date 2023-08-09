package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type QoS2 struct {
	ctx         context.Context
	cancel      context.CancelFunc
	meta        *meta
	queue       *QoS2Queue
	publishChan chan *packet.PublishMessage
	*StoreHelp
}

func NewQos2(topic string, writer PublishWriter, help *StoreHelp, data PrepareMetaData) *QoS2 {
	latestMessageID, _ := data.ReadTopicLatestPushedMessageID(topic)
	t := &QoS2{
		meta: &meta{
			topic:           topic,
			qos:             pkg.QoS1,
			writer:          writer,
			latestMessageID: latestMessageID,
		},
		queue:     NewQoS2Queue(writer),
		StoreHelp: help,
	}
	return t
}

func (q *QoS2) Start(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
	if q.meta.windowSize == 0 {
		q.meta.windowSize = config.GetTopic().WindowSize
	}
	q.publishChan = make(chan *packet.PublishMessage, q.meta.windowSize)

	// read client unRec packet first
	// read client unComp packet first
	q.pushMessage()
	// waiting for exit, prevent pushMessage use goroutine
	<-ctx.Done()
	if err := q.Close(); err != nil {
		logger.Logger.Warn("QoS1: close error = ", zap.Error(err))
	}
	if err := q.afterClose(); err != nil {
		logger.Logger.Warn("QoS1: after close error = ", zap.Error(err))
	}
}

func (q *QoS2) pushMessage() {
	for {
		select {
		case <-q.ctx.Done():
			return
		case msg, ok := <-q.publishChan:
			if !ok {
				return
			}
			msg.Packet.QoS = pkg.QoS2
			q.meta.writer.WritePacket(msg.Packet)
			q.queue.WritePacket(msg)
		default:
			if err := q.StoreHelp.readStore(q.ctx, q.meta.topic, q.meta.latestMessageID, q.meta.windowSize, false, q.writeToPublishChan); err != nil {
				logger.Logger.Error("QoS2: read store error = ", zap.Error(err), zap.String("store", q.meta.topic))
			}
		}
	}
}

func (q *QoS2) writeToPublishChan(message *packet.PublishMessage) {
	select {
	case q.publishChan <- message:
		q.meta.latestMessageID = message.MessageID
	default:

	}
}

func (q *QoS2) readUnRecPacket() {
	// TODO: read unRec packet
}
func (q *QoS2) readUnCompPacket() {
	// TODO: implement read unComp packet
}
func (q *QoS2) Close() error {
	q.cancel()
	return nil
}

func (q *QoS2) HandlePublishAck(puback *packets.Puback) {
	return
}

func (q *QoS2) HandlePublishRec(pubrec *packets.Pubrec) {
	q.queue.HandlePublishRec(pubrec)
}

func (q *QoS2) HandelPublishComp(pubcomp *packets.Pubcomp) {
	q.queue.HandlePublishComp(pubcomp)
}

func (q *QoS2) afterClose() error {
	return nil
}

func (q *QoS2) GetUnFinish() []*UnFinishPacket {
	return q.queue.getUnFinishPacket()
}

func (q *QoS2) GetUnCompPacketID() []string {
	return q.queue.getUnCompPacketID()
}
