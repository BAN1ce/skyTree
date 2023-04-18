package app

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/cluster"
	"github.com/BAN1ce/skyTree/inner/store"
	"github.com/lni/dragonboat/v3/config"
	"sync"
)

type App struct {
	node   *cluster.Node
	mux    sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewApp() *App {
	// todo: get config
	var (
		app = &App{}
	)
	app.node = cluster.NewNode([]cluster.Option{
		cluster.WithHostConfig(config.NodeHostConfig{
			RTTMillisecond: 1000,
			RaftAddress:    "localhost:9526",
			NodeHostDir:    "./data",
		}),
		cluster.WithClusterConfig(config.Config{
			NodeID:             1,
			ClusterID:          1,
			ElectionRTT:        100,
			HeartbeatRTT:       1,
			CheckQuorum:        true,
			SnapshotEntries:    100000,
			CompactionOverhead: 5,
		}),
		cluster.WithJoin(false),
		cluster.WithInitMembers(map[uint64]string{
			1: "localhost:9526",
		}),
		// todo : should support config
		cluster.WithStateMachine(store.NewCoreModel()),
	}...)
	return app
}

func (a *App) Start(ctx context.Context) error {
	return a.node.Start()
}
