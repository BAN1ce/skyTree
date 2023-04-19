package broker

/*
| Control Packet Type | Value |
|---------------------|-------|
| CONNECT             | 1     |
| CONNACK             | 2     |
| PUBLISH             | 3     |
| PUBACK              | 4     |
| PUBREC              | 5     |
| PUBREL              | 6     |
| PUBCOMP             | 7     |
| SUBSCRIBE           | 8     |
| SUBACK              | 9     |
| UNSUBSCRIBE         | 10    |
| UNSUBACK            | 11    |
| PINGREQ             | 12    |
| PINGRESP            | 13    |
| DISCONNECT          | 14    |
| AUTH                | 15    |
+---------------------+-------+
*/

type Core struct {
}

func (c *Core) Connect() {
	panic("implement me")
}

func (c *Core) Publish() {
	panic("implement me")
}

func (c *Core) PubRec() {
	panic("implement me")
}

func (c *Core) Subscribe() {
	panic("implement me")
}

func (c *Core) Unsubscribe() {
	panic("implement me")
}

func (c *Core) Ping() {
	panic("implement me")
}

func (c *Core) Disconnect() {
	panic("implement me")
}

func (c *Core) Auth() {
	panic("implement me")
}
