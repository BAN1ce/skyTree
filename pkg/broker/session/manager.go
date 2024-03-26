package session

import "context"

type Manager interface {
	ReadClientSession(ctx context.Context, clientID string) (Session, bool)
	AddClientSession(ctx context.Context, clientID string, session Session)
	NewClientSession(ctx context.Context, clientID string) Session
}
