package client

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/plugin"
	"github.com/BAN1ce/skyTree/pkg/broker/retain"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"time"
)

type NotifyClientClose interface {
	NotifyClientClose(c *Client)
	NotifyWillMessage(message *session.WillMessage)
}

type Component func(*component)

type component struct {
	Store       broker.TopicMessageStore
	session     session.Session
	retain      retain.Retain
	cfg         Config
	notifyClose NotifyClientClose
	plugin      *plugin.Plugins
}

func WithRetain(retain2 retain.Retain) Component {
	return func(options *component) {
		options.retain = retain2
	}
}

func WithStore(store broker.TopicMessageStore) Component {
	return func(options *component) {
		options.Store = store
	}
}
func WithConfig(cfg Config) Component {
	return func(options *component) {
		options.cfg = cfg
	}
}

func WithNotifyClose(notifyClose NotifyClientClose) Component {
	return func(options *component) {
		options.notifyClose = notifyClose
	}
}

func WithPlugin(plugins *plugin.Plugins) Component {
	return func(options *component) {
		options.plugin = plugins
	}
}

func WithSession(session2 session.Session) Component {
	return func(options *component) {
		options.session = session2
	}
}

func WithKeepAliveTime(keepAlive time.Duration) Component {
	return func(options *component) {
		options.cfg.KeepAlive = keepAlive
	}
}
