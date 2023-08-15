package client

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
)

type NotifyClientClose interface {
	NotifyClientClose(c *Client)
}
type Option func(*Options)

type Options struct {
	Store       broker.ClientMessageStore
	session     broker.Session
	cfg         Config
	notifyClose NotifyClientClose
}

func WithStore(store broker.ClientMessageStore) Option {
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
