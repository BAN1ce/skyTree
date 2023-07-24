package app

import (
	"context"
	"fmt"
	app2 "github.com/BAN1ce/Tree/app"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/api"
	"github.com/BAN1ce/skyTree/inner/broker"
	"github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/inner/broker/subtree"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/inner/metric"
	"github.com/BAN1ce/skyTree/inner/version"
	"github.com/nutsdb/nutsdb"
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
		subTree = app2.NewApp()
	)
	if err := subTree.StartTopicCluster(context.TODO()); err != nil {
		log.Fatal("start topic cluster failed", err)
	}
	var (
		clientManager  = broker.NewManager()
		sessionManager = session.NewSessionManager()
		dbStore        = store.NewLocalStore(nutsdb.Options{
			EntryIdxMode: nutsdb.HintKeyValAndRAMIdxMode,
			SegmentSize:  nutsdb.MB * 256,
			NodeNum:      1,
			RWMode:       nutsdb.MMap,
			SyncEnable:   true,
		}, nutsdb.WithDir("./data/nutsdb"))
		publishRetrySchedule = facade.SinglePublishRetry()
		app                  = &App{
			brokerCore: broker.NewBroker(
				broker.WithPublishRetry(publishRetrySchedule),
				broker.WithStore(store.NewStoreWrapper(dbStore)),
				broker.WithSessionManager(sessionManager),
				broker.WithClientManager(clientManager),
				broker.WithSubTree(subtree.NewSubTree(subTree)),
				broker.WithHandlers(&broker.Handlers{
					Connect:     broker.NewConnectHandler(),
					Publish:     broker.NewPublishHandler(),
					PublishAck:  broker.NewPublishAck(),
					PublishRec:  broker.NewPublishRec(),
					PublishRel:  broker.NewPublishRel(),
					PublishComp: broker.NewPublishComp(),
					Ping:        broker.NewPingHandler(),
					Sub:         broker.NewSubHandler(),
					UnSub:       broker.NewUnsubHandler(),
					Auth:        broker.NewAuthHandler(),
					Disconnect:  broker.NewDisconnectHandler(),
				})),
		}
	)
	app.components = []component{
		app.brokerCore,
		api.NewAPI(fmt.Sprintf(":%d", config.GetServer().GetPort()),
			api.WithClientManager(clientManager),
			api.WithStore(dbStore),
			api.WithSessionManager(sessionManager)),
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
