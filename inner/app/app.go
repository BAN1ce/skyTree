package app

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/api"
	"github.com/BAN1ce/skyTree/inner/broker"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/inner/cluster"
	"log"
	"sync"
)

type component interface {
	Start(ctx context.Context) error
	Name() string
}
type App struct {
	node       *cluster.Node
	mux        sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	brokerCore *broker.Core
	components []component
}

func NewApp() *App {
	// todo: get config
	var (
		clientManager = client.NewManager()
		app           = &App{
			brokerCore: broker.NewCore(broker.WithClientManager(clientManager), broker.WithSubTree(broker.NewSubTree())),
		}
	)
	app.components = []component{
		app.brokerCore,
		api.NewAPI(":9527", api.WithClientManager(clientManager)),
	}

	// app.node = cluster.NewNode([]cluster.Option{
	// 	cluster.WithHostConfig(config.NodeHostConfig{
	// 		RTTMillisecond: 1000,
	// 		RaftAddress:    "localhost:9526",
	// 		NodeHostDir:    "./data",
	// 	}),
	// 	cluster.WithClusterConfig(config.Config{
	// 		NodeID:             1,
	// 		ClusterID:          1,
	// 		ElectionRTT:        100,
	// 		HeartbeatRTT:       1,
	// 		CheckQuorum:        true,
	// 		SnapshotEntries:    100000,
	// 		CompactionOverhead: 5,
	// 	}),
	// 	cluster.WithJoin(false),
	// 	cluster.WithInitMembers(map[uint64]string{
	// 		1: "localhost:9526",
	// 	}),
	// 	// todo : should support config
	// 	cluster.WithStateMachine(meta.NewCoreModel()),
	// }...)
	return app
}

func (a *App) Start(ctx context.Context) {
	for _, c := range a.components {
		go func(c component) {
			if err := c.Start(ctx); err != nil {
				log.Fatalln("start component", c.Name(), "error: ", err)
			}
		}(c)
	}
}
