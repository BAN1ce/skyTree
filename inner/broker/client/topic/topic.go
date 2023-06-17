package topic

import "context"

type Topic interface {
	Start(ctx context.Context)
	Close() error
}
