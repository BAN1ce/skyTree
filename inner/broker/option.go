package broker

import (
	"github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/middleware"
)

type Option func(*Broker)

func WithUserAuth(auth middleware.UserAuth) Option {
	return func(core *Broker) {
		core.userAuth = auth
	}
}

func WithClientManager(manager *Manager) Option {
	return func(core *Broker) {
		core.clientManager = manager
	}
}

func WithSubTree(tree pkg.SubTree) Option {
	return func(core *Broker) {
		core.subTree = tree
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

func WithSessionManager(manager *session.Manager) Option {
	return func(broker *Broker) {
		broker.sessionManager = manager
	}
}

func WithStore(store pkg.Store) Option {
	return func(broker *Broker) {
		broker.store = store
	}
}

func WithPublishRetry(schedule facade.RetrySchedule) Option {
	return func(broker *Broker) {
		broker.publishRetry = schedule
	}
}
