package client

import (
	"sync"

	"github.com/BAN1ce/skyTree/pkg/metric"
)

type ID = string

type Manager struct {
	clients map[ID]*Client
	mux     sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		clients: map[ID]*Client{},
	}
}

// AddClient creates a new client and returns the client
// if the client already exists,
// it will return the old client and return false
// if the client is created successfully,
// it will return the new client and return true
func (c *Manager) AddClient(clientID string, client *Client) (oldClient *Client) {
	c.mux.Lock()
	defer c.mux.Unlock()

	oldClient = c.clients[clientID]
	c.clients[clientID] = client
	metric.ClientOnline.Set(float64(len(c.clients)))

	return
}

func (c *Manager) DeleteClient(client *Client) (*Client, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	cl, ok := c.clients[client.GetID()]
	if cl == client {
		delete(c.clients, client.GetID())
	}

	metric.ClientOnline.Set(float64(len(c.clients)))

	return cl, ok
}

func (c *Manager) ReadClient(id string) (*Client, bool) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	client2, ok := c.clients[id]
	return client2, ok
}

func (c *Manager) Snapshot() []*Client {
	c.mux.RLock()
	defer c.mux.RUnlock()

	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		clients = append(clients, client)
	}
	return clients
}
