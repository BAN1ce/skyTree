package broker

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

type ClientManager struct {
	mux     sync.RWMutex
	clients map[string]*Client
	event   chan *event
}

func newClientManager() *ClientManager {
	var (
		c = &ClientManager{
			clients: map[string]*Client{},
			event:   make(chan *event, 100),
		}
	)
	go c.listenClose()
	return c
}

func (c *ClientManager) CreateClient(client *Client) {
	c.event <- &event{
		client:    client,
		eventType: eventTypeAddClient,
	}
}

func (c *ClientManager) DeleteClient(client *Client) {
	c.event <- &event{
		client:    client,
		eventType: eventTypeRemClient,
	}
}

func (c *ClientManager) listenClose() {
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

func (c *ClientManager) createClient(client *Client) {
	if tmp, ok := c.clients[client.ID]; ok {
		if tmp != client {
			tmp.Close()
		}
	}
	c.clients[client.ID] = client
}

// deleteClient delete client from client manager and close client
func (c *ClientManager) deleteClient(client *Client) {
	if tmp, ok := c.clients[client.ID]; ok {
		if tmp == client {
			client.Close()
			delete(c.clients, client.ID)
			return
		}
	}
}

func (c *ClientManager) Write(clientID string, packet packets.Packet) (int64, error) {
	if client, ok := c.clients[clientID]; ok {
		return packet.WriteTo(client)
	}
	logger.Logger.Info("client not found = ", clientID)
	return 0, nil
}
