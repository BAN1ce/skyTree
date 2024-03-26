package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/broker/share"
	"github.com/BAN1ce/skyTree/inner/broker/state"
	"github.com/BAN1ce/skyTree/inner/broker/store/message"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/plugin"
	"github.com/BAN1ce/skyTree/pkg/broker/retain"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/BAN1ce/skyTree/pkg/broker/store"
	"github.com/BAN1ce/skyTree/pkg/middleware"
)

type Options struct {
}

type Option func(*Broker)

func WithShareManager(manager *share.Manager) Option {
	return func(broker *Broker) {
		broker.shareManager = manager
	}
}

func WithUserAuth(auth middleware.UserAuth) Option {
	return func(core *Broker) {
		core.userAuth = auth
	}
}

func WithClientManager(manager *client.Clients) Option {
	return func(core *Broker) {
		core.clientManager = manager
	}
}

func WithSubCenter(tree broker.SubCenter) Option {
	return func(core *Broker) {
		core.subTree = tree
	}
}

func WithState(s *state.State) Option {
	return func(core *Broker) {
		core.state = s
	}
}

func AppendMiddleware(packet byte, handle ...middleware.PacketMiddleware) Option {
	return func(core *Broker) {
		core.preMiddleware[packet] = append(core.preMiddleware[packet], handle...)
	}
}

func WithHandlers(handlers *Handlers) Option {
	return func(broker *Broker) {
		broker.handlers = handlers
	}
}

func WithSessionManager(manager session.Manager) Option {
	return func(broker *Broker) {
		broker.sessionManager = manager
	}
}

func WithMessageStore(store *message.Wrapper) Option {
	return func(broker *Broker) {
		broker.messageStore = store
	}
}

func WithPublishRetry(schedule facade.RetrySchedule) Option {
	return func(broker *Broker) {
		broker.publishRetry = schedule
	}
}

func WithPlugins(plugins *plugin.Plugins) Option {
	return func(broker *Broker) {
		broker.plugins = plugins
	}
}

func WithKeyStore(store store.KeyStore) Option {
	return func(b *Broker) {
		b.keyStore = store
	}
}

func WithRetainStore(retain retain.Retain) Option {
	return func(b *Broker) {
		b.retain = retain
	}

}
