package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/eclipse/paho.golang/packets"
)

type PublishRec struct {
}

func NewPublishRec() *PublishRec {
	return &PublishRec{}
}

func (p *PublishRec) Handle(broker *Broker, client *client.Client, rawPacket *packets.ControlPacket) (err error) {
	return nil
	//var (
	//	packet, ok = rawPacket.Content.(*packets.Pubrec)
	//)
	//if !ok {
	//	logger.Logger.Error("convert to pubrec error")
	//	return err
	//}
	//return err
}
