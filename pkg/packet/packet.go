package packet

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
const (
	CONNECT = 1 << iota
	CONNACK
	PUBLISH
	PUBACK
	PUBREC
	PUBREL
	PUBCOMP
	SUBSCRIBE
	SUBACK
	UNSUBSCRIBE
	UNSUBACK
	PINGREQ
	PINGRESP
	DISCONNECT
	AUTH
)
