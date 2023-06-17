package config

import (
	"context"
	"sync"
)

var (
	rootContextOnce sync.Once
	rootContext     context.Context
)

func GetRootContext() context.Context {
	rootContextOnce.Do(func() {
		rootContext = context.Background()
	})
	return rootContext
}
