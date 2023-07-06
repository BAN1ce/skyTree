package app

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/api"
	"github.com/BAN1ce/skyTree/inner/broker"
	"github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/inner/metric"
	"github.com/BAN1ce/skyTree/inner/version"
	"log"
	"sync"
)

type component interface {
	Start(ctx context.Context) error
	Name() string
}

type App struct {
	ctx        context.Context
	cancel     context.CancelFunc
	brokerCore *broker.Broker
	components []component
	mux        sync.Mutex
}

func NewApp() *App {
	// todo: get config
	metric.Init()
	var (
		clientManager        = broker.NewManager()
		sessionManager       = session.NewSessionManager()
		dbStore              = store.NewNutsDBStore()
		publishRetrySchedule = facade.SinglePublishRetry()
		app                  = &App{
			brokerCore: broker.NewBroker(
				broker.WithPublishRetry(publishRetrySchedule),
				broker.WithStore(store.NewStoreWrapper(dbStore)),
				broker.WithSessionManager(sessionManager),
				broker.WithClientManager(clientManager),
				broker.WithSubTree(broker.NewSubTree()),
				broker.WithHandlers(&broker.Handlers{
					Connect:    broker.NewConnectHandler(),
					Publish:    broker.NewPublishHandler(),
					PublishAck: broker.NewPublishAck(),
					PublishRel: broker.NewPublishRel(),
					Ping:       broker.NewPingHandler(),
					Sub:        broker.NewSubHandler(),
					UnSub:      broker.NewUnsubHandler(),
					Auth:       broker.NewAuthHandler(),
					Disconnect: broker.NewDisconnectHandler(),
				})),
		}
	)
	app.components = []component{
		app.brokerCore,
		api.NewAPI(fmt.Sprintf(":%d", config.GetServer().GetPort()), api.WithClientManager(clientManager), api.WithStore(dbStore)),
	}
	return app
}

func (a *App) Start(ctx context.Context) {
	a.mux.Lock()
	defer a.mux.Unlock()
	a.ctx = ctx
	for _, c := range a.components {
		go func(c component) {
			if err := c.Start(ctx); err != nil {
				log.Fatalln("start component", c.Name(), "error: ", err)
			}
		}(c)
	}
}

func (a *App) Stop() {
	a.mux.Lock()
	defer a.mux.Unlock()
	a.cancel()
}

func (a *App) Version() string {
	return version.Version
}
