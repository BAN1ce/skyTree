package client

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

func (c *Client) DoSendConnAckPlugin(connAck *packets.Connack) {
	if c.component.plugin == nil || c.component.plugin.OnSendConnAck == nil {
		return
	}
	for _, p := range c.component.plugin.OnSendConnAck {
		if err := p(c.UID, connAck); err != nil {
			logger.Logger.Error("plugin OnSendConnAck error", zap.Error(err), zap.String("client", c.MetaString()))
		}
	}
}
