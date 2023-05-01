package pkg

type SessionKey string

const (
	WillMessage   = SessionKey("will_message")
	WillQos       = SessionKey("will_qos")
	WillRetain    = SessionKey("will_retain")
	Username      = SessionKey("username")
	KeepAlive     = SessionKey("keep_alive")
	LastAliveTime = SessionKey("last_alive_time")
)

type Session interface {
	Get(key SessionKey) string
	Set(key SessionKey, value string)
	Destroy()
}
