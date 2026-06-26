package pkg

import (
	"context"
	"github.com/google/uuid"
)

type contextKey string

var (
	ContextIDKey contextKey = "context_id"
	ClientIDKey  contextKey = "client_id"
)

func NewCtxWithID() context.Context {
	return context.WithValue(context.Background(), ContextIDKey, uuid.NewString())
}

func GetContextID(ctx context.Context) (id string) {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(ContextIDKey).(string); ok {
		return id
	}
	return ""
}

func SetClientID(ctx context.Context, clientID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ClientIDKey, clientID)

}

type GetID interface {
	GetID() string
}

func GetClientID(ctx context.Context) (id string) {
	if ctx == nil {
		return ""
	}
	switch v := ctx.Value(ClientIDKey).(type) {
	case string:
		return v
	case GetID:
		return v.GetID()
	}
	return ""
}
