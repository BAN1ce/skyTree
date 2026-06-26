package bootstrap

import (
	"io"

	"github.com/BAN1ce/skyTree/internal/app/brokerruntime"
	"github.com/BAN1ce/skyTree/internal/app/clusterruntime"
	"github.com/BAN1ce/skyTree/internal/app/lifecycle"
	"github.com/BAN1ce/skyTree/internal/app/serverruntime"
	"github.com/BAN1ce/skyTree/internal/app/statecenter"
	"github.com/BAN1ce/skyTree/internal/app/storeruntime"
	delivery_event "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	inner_cluster "github.com/BAN1ce/skyTree/internal/clusterhealth"
	"github.com/BAN1ce/skyTree/pkg/eventbus"
	"github.com/kataras/go-events"
)

// AppRuntime 聚合 App 启动后需要持有的各类运行时依赖。
type AppRuntime struct {
	LocalEvent       events.EventEmmiter
	LocalEventCenter *eventbus.EventCenter[*delivery_event.Notify]

	Cluster      *clusterruntime.Runtime
	Stores       *storeruntime.Runtime
	StateCenters *statecenter.Runtime
	Broker       *brokerruntime.Runtime
	Servers      *serverruntime.Runtime

	HealthChecker *inner_cluster.HealthChecker
	Runner        *lifecycle.Runner
	resources     []io.Closer
}

// CloseResources 关闭由 AppRuntime 持有的底层资源，按创建顺序反向执行。
func (r *AppRuntime) CloseResources() error {
	if r == nil {
		return nil
	}
	resources := r.resources
	r.resources = nil
	return closeAllReverse(resources)
}
