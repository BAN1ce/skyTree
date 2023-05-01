package client

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/state"
	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"
	"net"
	"strconv"
	"sync"
)

type Handler interface {
	ListenClientClose(*Client)
	HandlePacket(*Client, *packets.ControlPacket)
}

type Client struct {
	mux               sync.RWMutex
	conn              net.Conn
	ID                string
	handler           Handler
	closeOnce         sync.Once
	state             state.State
	session           pkg.Session
	connectProperties paho.ConnectProperties
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn: conn,
	}
}

func (c *Client) Write(data []byte) (int, error) {
	return c.conn.Write(data)
}

func (c *Client) Run(handler Handler) {
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
			handler.HandlePacket(c, packet)
		}
	}()
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		logger.Logger.Info("client close = ", c.ID)
		if err := c.conn.Close(); err != nil {
			logger.Logger.Error("close client error = ", err.Error())
		}
		// TODO: check will message
		c.handler.ListenClientClose(c)
	})

}

func (c *Client) SetID(id string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.ID = id
}

func (c *Client) SetState(state uint64) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.state.SetState(state)
}

func (c *Client) IsState(state uint64) bool {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.state.IsState(state)
}

func (c *Client) RemState(state uint64) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.state.RemState(state)
}

func (c *Client) WritePacket(packet packets.Packet) {
	if _, err := packet.WriteTo(c); err != nil {
		logger.Logger.Info("write packet error = ", err.Error())
		c.Close()
	}
}

func (c *Client) SetSession(session pkg.Session) {
	c.mux.Lock()
	c.session = session
	c.mux.Unlock()
}

func (c *Client) GetValue(key pkg.SessionKey) string {
	c.mux.RLock()
	tmp := c.session.Get(key)
	c.mux.RUnlock()
	return tmp
}

func (c *Client) SetValue(key pkg.SessionKey, value string) {
	c.mux.Lock()
	c.session.Set(key, value)
	c.mux.Unlock()
}

func (c *Client) DestroySession() {
	c.mux.Lock()
	c.session.Destroy()
	c.mux.Unlock()
}

func (c *Client) SetLastAliveTime(time int64) {
	c.mux.Lock()
	c.session.Set(pkg.LastAliveTime, strconv.FormatInt(time, 10))
	c.mux.Unlock()
}

func (c *Client) SetWillMessage(message string) {
	c.mux.Lock()
	c.session.Set(pkg.WillMessage, message)
	c.mux.Unlock()
}
