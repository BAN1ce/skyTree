package client

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/plugin"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"time"
)

type NotifyClientClose interface {
	NotifyClientClose(c *Client)
	NotifyWillMessage(message *session.WillMessage)
}

type Option func(*Options)

type Options struct {
	Store       broker.TopicMessageStore
	session     session.Session
	cfg         Config
	notifyClose NotifyClientClose
	plugin      *plugin.Plugins
}

func WithStore(store broker.TopicMessageStore) Option {
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

func WithPlugin(plugins *plugin.Plugins) Option {
	return func(options *Options) {
		options.plugin = plugins
	}
}

func WithSession(session2 session.Session) Option {
	return func(options *Options) {
		options.session = session2
	}
}

func WithKeepAliveTime(keepAlive time.Duration) Option {
	return func(options *Options) {
		options.cfg.KeepAlive = keepAlive
	}
}
