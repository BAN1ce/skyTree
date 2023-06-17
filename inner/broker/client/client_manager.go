package client

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
)

type Manager struct {
	clients map[string]*Client
}

func NewManager() *Manager {
	var (
		c = &Manager{
			clients: map[string]*Client{},
		}
	)
	return c
}

func (c *Manager) CreateClient(client *Client) {
	c.createClient(client)
}

func (c *Manager) DeleteClient(client *Client) {
	c.deleteClient(client)
}

func (c *Manager) createClient(client *Client) {
	if tmp, ok := c.clients[client.ID]; ok {
		if tmp != client {
			tmp.Close()
		}
	}
	c.clients[client.ID] = client
}

// deleteClient delete client from client manager and close client
func (c *Manager) deleteClient(client *Client) {
	if tmp, ok := c.clients[client.ID]; ok {
		if tmp == client {
			client.Close()
			delete(c.clients, client.ID)
			return
		}
	}
}

func (c *Manager) ReadClient(clientID string) (*Client, bool) {
	if client, ok := c.clients[clientID]; ok {
		return client, true
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
