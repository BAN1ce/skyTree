package client

import "github.com/BAN1ce/skyTree/pkg"

type Option func(*Options)
type Options struct {
	Store   pkg.ClientMessageStore
	session pkg.Session
	cfg     *Config
}

func WithStore(store pkg.ClientMessageStore) Option {
	return func(options *Options) {
		options.Store = store
	}
}
func WithConfig(cfg *Config) Option {
	return func(options *Options) {
		options.cfg = cfg
	}
}
