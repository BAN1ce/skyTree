package app

import (
	"context"
	"fmt"
	"github.com/BAN1ce/skyTree/api"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/inner/broker/core"
	"github.com/BAN1ce/skyTree/inner/broker/session"
	"github.com/BAN1ce/skyTree/inner/broker/share"
	"github.com/BAN1ce/skyTree/inner/broker/state"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/inner/broker/store/key"
	"github.com/BAN1ce/skyTree/inner/broker/store/message"
	"github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/inner/facade"
	"github.com/BAN1ce/skyTree/inner/metric"
	"github.com/BAN1ce/skyTree/inner/version"
	broker2 "github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/plugin"
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
	brokerCore *core.Broker
	components []component
	mux        sync.Mutex
}

func NewApp() *App {
	// todo: get config
	metric.Boot()
	//var (
	//	subTree = app2.NewApp()
	//)
	//if err := subTree.StartTopicCluster(context.TODO(), []app2.Option{
	//	app2.WithInitMember(config.GetTree().InitNode),
	//	app2.WithJoin(false),
	//	app2.WithNodeConfig(config.GetTree().NodeHostConfig),
	//	app2.WithConfig(config.GetTree().DragonboatConfig),
	//	app2.WithStateMachine(store2.NewState()),
	//}...); err != nil {
	//	log.Fatal("start store cluster failed", err)
	//}
	var (
		clientManager = core.NewClientManager()
	)
	var (
		localNutsDBStore = key.NewLocalStore(nutsdb.Options{
			EntryIdxMode: nutsdb.HintKeyValAndRAMIdxMode,
			SegmentSize:  nutsdb.MB * 256,
			NodeNum:      1,
			RWMode:       nutsdb.MMap,
			SyncEnable:   true,
		}, nutsdb.WithDir("./data/nutsdb"))

		keyVStore = localNutsDBStore
		//keyVStore = key.NewRedis()
	)
	var (
		sessionManager = session.NewSessions(keyVStore)
	)
	event.Boot()
	store.Boot(localNutsDBStore, event.GlobalEvent)
	var (
		storeWrapper     = message.NewStoreWrapper(localNutsDBStore, event.GlobalEvent)
		shareTopicManger = share.NewManager(storeWrapper)
	)

	// TODO: fix this
	//event.GlobalEvent.CreateListenClientOffline(shareTopicManger.CloseClient)

	var (
		publishRetrySchedule = facade.SinglePublishRetry()
		app                  = &App{
			brokerCore: core.NewBroker(
				core.WithKeyStore(keyVStore),
				core.WithPlugins(plugin.NewDefaultPlugin()),
				core.WithPublishRetry(publishRetrySchedule),
				core.WithMessageStore(storeWrapper),
				core.WithState(state.NewState(keyVStore)),
				core.WithSessionManager(sessionManager),
				core.WithClientManager(clientManager),
				core.WithShareManager(shareTopicManger),
				core.WithSubCenter(broker2.NewLocalSubCenter()),
				core.WithHandlers(&core.Handlers{
					Connect:     core.NewConnectHandler(),
					Publish:     core.NewPublishHandler(),
					PublishAck:  core.NewPublishAck(),
					PublishRec:  core.NewPublishRec(),
					PublishRel:  core.NewPublishRel(),
					PublishComp: core.NewPublishComp(),
					Ping:        core.NewPingHandler(),
					Sub:         core.NewSubHandler(),
					UnSub:       core.NewUnsubHandler(),
					Auth:        core.NewAuthHandler(),
					Disconnect:  core.NewDisconnectHandler(),
				})),
		}
	)
	app.components = []component{
		app.brokerCore,
		api.NewAPI(fmt.Sprintf(":%d", config.GetServer().GetPort()),
			api.WithClientManager(clientManager),
			api.WithStore(localNutsDBStore),
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
