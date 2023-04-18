package cluster

import (
	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/statemachine"
)

type Option func(*Node)

func WithHostConfig(cfg config.NodeHostConfig) Option {
	return func(n *Node) {
		n.hostCfg = cfg
	}
}

func WithClusterConfig(cfg config.Config) Option {
	return func(n *Node) {
		n.clusterCfg = cfg
	}
}

func WithInitMembers(members map[uint64]string) Option {
	return func(n *Node) {
		n.initMembers = members
	}
}

func WithJoin(join bool) Option {
	return func(n *Node) {
		n.join = join
	}
}

func WithStateMachine(state statemachine.IStateMachine) Option {
	return func(n *Node) {
		n.state = state
	}
}

type Node struct {
	hostCfg     config.NodeHostConfig
	clusterCfg  config.Config
	initMembers map[uint64]string
	join        bool
	state       statemachine.IStateMachine
	nodeHost    *dragonboat.NodeHost
}

func NewNode(option ...Option) *Node {
	var n = &Node{}
	for _, opt := range option {
		opt(n)
	}
	return n
}

func (n *Node) Start() error {
	var (
		err error
	)
	n.nodeHost, err = dragonboat.NewNodeHost(n.hostCfg)
	if err != nil {
		return err
	}
	return n.nodeHost.StartCluster(n.initMembers, n.join, func(u uint64, u2 uint64) statemachine.IStateMachine {
		return n.state
	}, n.clusterCfg)
}

func (n *Node) Close() error {
	return nil
}
