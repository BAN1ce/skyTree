package topic

import (
	"github.com/BAN1ce/skyTree/pkg/broker/client"
	"github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
)

type Client struct {
	writer client.PacketWriter
	meta   *topic.Meta
}

func NewClient(writer client.PacketWriter, meta *topic.Meta) *Client {
	return &Client{
		writer: writer,
		meta:   meta,
	}
}

func (c *Client) Publish(publish *packet.Message) error {
	// NoLocal means that the server does not send messages published by the client itself.
	if c.meta.NoLocal && c.writer.GetID() == publish.ClientID {
		return nil
	}
	var (
		publishPacket = pool.PublishPool.Get()
	)
	defer pool.PublishPool.Put(publishPacket)

	pool.CopyPublish(publishPacket, publish.PublishPacket)
	publishPacket.QoS = byte(c.meta.QoS)

	// if topic has identifier, set the identifier to the publishing packet
	if c.meta.Identifier != 0 {
		identifier := byte(c.meta.Identifier)
		if publishPacket.Properties == nil {
			publishPacket.Properties = &packets.Properties{}
		}
		publishPacket.Properties.SubIDAvailable = &identifier
	}

	return c.writer.WritePacket(publishPacket)
}

func (c *Client) PubRel(message *packet.Message) error {
	return c.writer.WritePacket(message.PubRelPacket)
}

func (c *Client) GetPacketWriter() client.PacketWriter {
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
