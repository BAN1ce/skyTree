package broker

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"net"
)

type ClientHandler func(*Client, *packets.ControlPacket)

type Client struct {
	conn    net.Conn
	ID      string
	handler ClientHandler
	user    []packets.User
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
	c.handler = handler
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
			c.handler(c, packet)
		}
	}()
}

func (c *Client) Close() {
	if err := c.conn.Close(); err != nil {
		logger.Logger.Error("close client error = ", err.Error())
	}
}
