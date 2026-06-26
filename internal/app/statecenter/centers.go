package statecenter

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/config"
	willdelay "github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
)

// Runtime 保存 broker 依赖的状态中心集合。
type Runtime struct {
	Subscription subscription.Center
	Session      session.Center
	WillDelay    willdelay.Center
}

// Build 根据运行模式构建订阅、会话和 will-delay 状态中心。
func Build(ctx context.Context, cfg config.AppConfig, cluster *raft2.Cluster) (*Runtime, error) {
	subCenter, err := BuildSubscriptionCenter(ctx, cfg, cluster)
	if err != nil {
		return nil, fmt.Errorf("build sub center failed: %w", err)
	}

	sessionCenter, err := BuildSessionCenter(ctx, cfg, cluster)
	if err != nil {
		return nil, fmt.Errorf("build session center failed: %w", err)
	}

	willDelayCenter, err := BuildWillDelayCenter(ctx, cfg, cluster)
	if err != nil {
		return nil, fmt.Errorf("build will delay center failed: %w", err)
	}

	return &Runtime{
		Subscription: subCenter,
		Session:      sessionCenter,
		WillDelay:    willDelayCenter,
	}, nil
}
