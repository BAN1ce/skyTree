package topic

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker/client"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/eclipse/paho.golang/packets"
	"github.com/zyedidia/generic/list"
	"go.uber.org/zap"
)

type WithRetryClient struct {
	client *Client
	queue  *list.List[*packet.Message]
	retry  retry.MessageRetry
}

func NewQoSWithRetry(client *Client, messageRetry retry.MessageRetry) *WithRetryClient {
	return &WithRetryClient{
		client: client,
		queue:  list.New[*packet.Message](),
		retry:  messageRetry,
	}
}

func (r *WithRetryClient) Publish(publish *packet.Message) error {
	//var (
	//	retryKey = uuid.NewString()
	//)
	// TODO: create retry job
	r.queue.PushBack(publish)
	return r.client.Publish(publish)
}

func (r *WithRetryClient) HandlePublishAck(pubAck *packets.Puback) {
	node := r.queue.Front
	for node != nil {
		if node.Value.PublishPacket.PacketID == pubAck.PacketID {
			logger.Logger.Debug("WithRetryClient: remove publish message from queue", zap.Uint16("packetID", pubAck.PacketID))
			// TODO : delete retry task
			r.queue.Remove(node)
			break
		}
		node = node.Next
	}
	r.client.HandlePublishAck(pubAck)
}

func (r *WithRetryClient) GetPacketWriter() client.PacketWriter {
	return r.client.GetPacketWriter()
}

func (r *WithRetryClient) PubRel(message *packet.Message) error {
	// TODO: need retry ?
	return r.client.PubRel(message)
}

func (r *WithRetryClient) GetUnFinishedMessage() []*packet.Message {
	var (
		unFinish []*packet.Message
	)
	r.queue.Front.Each(func(val *packet.Message) {
		unFinish = append(unFinish, val)
	})
	return unFinish
}

func (r *WithRetryClient) HandlePublishRec(pubRec *packets.Pubrec) {
	var (
		node    = r.queue.Front
		message *packet.Message
	)
	for node != nil {
		if node.Value.PublishPacket.PacketID == pubRec.PacketID {
			message = node.Value
			logger.Logger.Debug("WithRetryClient: mark publish message be received", zap.Uint16("packetID", pubRec.PacketID))
			node.Value.PubReceived = true
			break
		}
		node = node.Next
	}
	r.client.HandlePublishRec(pubRec)
	pubRel := packet.NewPublishRel()
	pubRel.PacketID = pubRec.PacketID
	message.PubRelPacket = pubRel
	logger.Logger.Debug("WithRetryClient: send pubRel", zap.Uint16("packetID", pubRec.PacketID))
	if err := r.client.PubRel(message); err != nil {
		logger.Logger.Info("WithRetryClient: pubRel failed", zap.Error(err))
	}
}

func (r *WithRetryClient) HandelPublishComp(pubComp *packets.Pubcomp) {
	node := r.queue.Front
	for node != nil {
		if node.Value.PublishPacket.PacketID == pubComp.PacketID {
			logger.Logger.Debug("WithRetryClient: remove publish message from queue with pubComp", zap.Uint16("packetID", pubComp.PacketID))
			node.Value.PubReceived = true
			// TODO : delete retry task
			r.queue.Remove(node)
			break
		}
		node = node.Next
	}
	r.client.HandelPublishComp(pubComp)
}

func (r *WithRetryClient) Close() error {
	return nil
}
