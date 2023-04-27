package client

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"sync"
)

const (
	eventTypeAddClient = iota
	eventTypeRemClient
)

type event struct {
	eventType int
	client    *Client
}

type Manager struct {
	mux     sync.RWMutex
	clients map[string]*Client
	event   chan *event
}

func NewManager() *Manager {
	var (
		c = &Manager{
			clients: map[string]*Client{},
			event:   make(chan *event, 100),
		}
	)
	go c.listenClose()
	return c
}

func (c *Manager) CreateClient(client *Client) {
	c.event <- &event{
		client:    client,
		eventType: eventTypeAddClient,
	}
}

func (c *Manager) DeleteClient(client *Client) {
	c.event <- &event{
		client:    client,
		eventType: eventTypeRemClient,
	}
}

func (c *Manager) listenClose() {
	// TODO: graceful shutdown
	for event := range c.event {
		switch event.eventType {
		case eventTypeAddClient:
			c.createClient(event.client)
		case eventTypeRemClient:
			c.deleteClient(event.client)
		}
	}
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
	c.mux.RLock()
	defer c.mux.RUnlock()
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

func (c *Manager) Info() *Info {
	var (
		info = Info{
			Total: len(c.clients),
		}
	)
	return &info
}
