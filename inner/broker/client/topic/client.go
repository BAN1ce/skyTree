package topic

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
)

type Client struct {
	writer    broker.PacketWriter
	subOption *packets.SubOptions
}

func NewClient(writer broker.PacketWriter, subOption *packets.SubOptions) broker.Client {
	return &Client{
		writer:    writer,
		subOption: subOption,
	}
}

func (c *Client) Publish(publish *packet.Message) error {
	// NoLocal means that the server does not send messages published by the client itself.
	if c.subOption.NoLocal && c.writer.GetID() == publish.ClientID {
		return nil
	}
	var (
		publishPacket = pool.PublishPool.Get()
	)
	defer pool.PublishPool.Put(publishPacket)

	pool.CopyPublish(publishPacket, publish.PublishPacket)
	publishPacket.QoS = c.subOption.QoS

	return c.writer.WritePacket(publishPacket)
}

func (c *Client) PubRel(message *packet.Message) error {
	return c.writer.WritePacket(message.PubRelPacket)
}

func (c *Client) GetPacketWriter() broker.PacketWriter {
	return c.writer
}

func (c *Client) HandlePublishAck(pubAck *packets.Puback) {
	// do nothing
}

func (c *Client) HandlePublishRec(pubRec *packets.Pubrec) {
	// do nothing
}

func (c *Client) HandelPublishComp(pubComp *packets.Pubcomp) {
	// do nothing
}

func (c *Client) GetUnFinishedMessage() []*packet.Message {
	return nil
}
