package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

type ClientManager struct {
	clients map[string]*client.Client
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

func (c *ClientManager) DeleteClient(clientID string) {
	if client2, ok := c.clients[clientID]; ok {
		c.deleteClient(client2)
	}
}

func (c *ClientManager) createClient(client *client.Client) {
	if tmp, ok := c.clients[client.ID]; ok {
		if tmp != client {
			tmp.Close()
		}
	}
	c.clients[client.ID] = client
}

// deleteClient delete client from client manager and close client
func (c *ClientManager) deleteClient(client *client.Client) {
	if tmp, ok := c.clients[client.ID]; ok {
		if tmp == client {
			client.Close()
			delete(c.clients, client.ID)
			return
		}
	}
}

func (c *ClientManager) ReadClient(clientID string) (*client.Client, bool) {
	if client2, ok := c.clients[clientID]; ok {
		return client2, true
	}
	return nil, false
}

func (c *ClientManager) Write(clientID string, packet packets.Packet) {
	if client2, ok := c.clients[clientID]; ok {
		client2.WritePacket(packet)
	} else {
		logger.Logger.Info("client not found = ", zap.String("clientID", clientID))
	}
}
