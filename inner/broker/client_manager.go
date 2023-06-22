package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
)

type Manager struct {
	clients map[string]*client.Client
}

func NewManager() *Manager {
	var (
		c = &Manager{
			clients: map[string]*client.Client{},
		}
	)
	return c
}

func (c *Manager) CreateClient(client *client.Client) {
	c.createClient(client)
}

func (c *Manager) DeleteClient(clientID string) {
	if client2, ok := c.clients[clientID]; ok {
		c.deleteClient(client2)
	}
}

func (c *Manager) createClient(client *client.Client) {
	if tmp, ok := c.clients[client.ID]; ok {
		if tmp != client {
			tmp.Close()
		}
	}
	c.clients[client.ID] = client
}

// deleteClient delete client from client manager and close client
func (c *Manager) deleteClient(client *client.Client) {
	if tmp, ok := c.clients[client.ID]; ok {
		if tmp == client {
			client.Close()
			delete(c.clients, client.ID)
			return
		}
	}
}

func (c *Manager) ReadClient(clientID string) (*client.Client, bool) {
	if client2, ok := c.clients[clientID]; ok {
		return client2, true
	}
	return nil, false
}

func (c *Manager) Write(clientID string, packet packets.Packet) (int64, error) {
	if client, ok := c.clients[clientID]; ok {
		return packet.WriteTo(client)
	}
	logger.Logger.Info("client not found = ", clientID)
	return 0, nil
}
