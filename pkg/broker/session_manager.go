package broker

type SessionManager interface {
	ReadSession(key string) (Session, bool)
	DeleteSession(key string)
	CreateSession(key string, session Session)
	NewSession(key string) Session
}
