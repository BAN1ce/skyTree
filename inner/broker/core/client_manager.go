package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"go.uber.org/zap"
)

type UID = string
type ClientManager struct {
	clients map[UID]*client.Client
}

func NewClientManager() *ClientManager {
	var (
		c = &ClientManager{
			clients: map[string]*client.Client{},
		}
	)
	return c
}

func (c *ClientManager) CreateClient(client *client.Client) {
	c.createClient(client)
}

func (c *ClientManager) DeleteClient(uid string) {
	if client2, ok := c.clients[uid]; ok {
		c.deleteClient(client2)
	}
}

func (c *ClientManager) createClient(client *client.Client) {
	if tmp, ok := c.clients[client.UID]; ok {
		if tmp != client {
			logger.Logger.Warn("client already exist", zap.String("client uid", client.UID))
			if err := tmp.Close(); err != nil {
				logger.Logger.Warn("close old client error", zap.Error(err), zap.String("uid", client.UID))
			}
		}
	}
	c.clients[client.ID] = client
}

// deleteClient delete client from client manager and close client
func (c *ClientManager) deleteClient(client *client.Client) {
	if tmp, ok := c.clients[client.UID]; ok {
		if tmp == client {
			delete(c.clients, client.UID)
			if err := client.Close(); err != nil {
				logger.Logger.Warn("close client error", zap.Error(err))
			}
			// FIXME:
			// BUG
			return
		}
	}
}

func (c *ClientManager) ReadClient(uid string) (*client.Client, bool) {
	if client2, ok := c.clients[uid]; ok {
		return client2, true
	}
	return nil, false
}

//func (c *ClientManager) Write(uid string, packet packets.Packet) {
//	if client2, ok := c.clients[uid]; ok {
//		client2.WritePacket(packet)
//	} else {
//		logger.Logger.Info("client not found = ", zap.String("client uid", uid))
//	}
//}
