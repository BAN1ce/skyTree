package broker

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"net"
	"sync"
)

type ClientHandler interface {
	ListenClientClose(*Client)
	HandlePacket(*Client, *packets.ControlPacket)
}

type Client struct {
	conn net.Conn
	ID   string
	user []packets.User
	mux  sync.RWMutex
}

func newClient(conn net.Conn) *Client {
	return &Client{
		conn: conn,
	}
}

func (c *Client) Write(data []byte) (int, error) {
	return c.conn.Write(data)
}

func (c *Client) Run(handler ClientHandler) {
	var (
		packet *packets.ControlPacket
		err    error
	)
	go func() {
		for {
			packet, err = packets.ReadPacket(c.conn)
			if err != nil {
				logger.Logger.Error("read packet error = ", err.Error())
				c.Close()
				return
			}
			handler.HandlePacket(c, packet)
		}
	}()
}

func (c *Client) Close() {
	logger.Logger.Info("client close = ", c.ID)
	if err := c.conn.Close(); err != nil {
		logger.Logger.Error("close client error = ", err.Error())
	}
}

func (c *Client) SetID(id string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.ID = id
}
