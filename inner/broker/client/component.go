package client

import "github.com/BAN1ce/skyTree/pkg"

type Component func(*Components)

type Components struct {
	Store   pkg.ClientMessageStore
	session pkg.Session
	cfg     Config
}

func WithStore(store pkg.ClientMessageStore) Component {
	return func(options *Components) {
		options.Store = store
	}
}
func WithConfig(cfg Config) Component {
	return func(options *Components) {
		options.cfg = cfg
	}
}
