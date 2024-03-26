package client

import (
	"github.com/BAN1ce/skyTree/logger"
	"go.uber.org/zap"
	"sync"
)

type ID = string

type Clients struct {
	mux     sync.RWMutex
	clients map[ID]*Client
}

func NewClients() *Clients {
	return &Clients{
		clients: map[string]*Client{},
	}
}

func (c *Clients) CreateClient(client *Client) {
	c.createClient(client)
}

func (c *Clients) DeleteClient(uid string) {
	if client2, ok := c.clients[uid]; ok {
		c.deleteClient(client2)
	}
}

func (c *Clients) createClient(client *Client) {
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
func (c *Clients) deleteClient(client *Client) {
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

func (c *Clients) ReadClient(uid string) (*Client, bool) {
	if client2, ok := c.clients[uid]; ok {
		return client2, true
	}
	return nil, false
}
