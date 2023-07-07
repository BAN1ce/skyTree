package client

import (
	"github.com/BAN1ce/skyTree/pkg"
)

type NotifyClientClose interface {
	NotifyClientClose(c *Client)
}
type Option func(*Options)

type Options struct {
	Store       pkg.ClientMessageStore
	session     pkg.Session
	cfg         Config
	notifyClose NotifyClientClose
}

func WithStore(store pkg.ClientMessageStore) Option {
	return func(options *Options) {
		options.Store = store
	}
}
func WithConfig(cfg Config) Option {
	return func(options *Options) {
		options.cfg = cfg
	}
}

func WithNotifyClose(notifyClose NotifyClientClose) Option {
	return func(options *Options) {
		options.notifyClose = notifyClose
	}
}
