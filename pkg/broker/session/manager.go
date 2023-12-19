package session

type Manager interface {
	ReadClientSession(clientID string) (Session, bool)
	AddClientSession(clientID string, session Session)
	NewClientSession(clientID string) Session
}
