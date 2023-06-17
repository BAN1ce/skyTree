package client

const (
	ReceivedConnect = uint64(1 << iota)
)

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
