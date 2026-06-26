package client

import (
	"context"
	"testing"

	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
)

type testStateRouter struct {
	routePublish           func(context.Context, staterouter.RoutePublishRequest) error
	hasMatchingSubscribers func(context.Context, staterouter.HasMatchingSubscribersRequest) (bool, error)
	subscribe              func(context.Context, staterouter.SubscribeRequest) (*staterouter.SubscribeResponse, error)
}

func (r *testStateRouter) AcquireSession(
	context.Context,
	staterouter.AcquireSessionRequest,
) (*staterouter.AcquireSessionResponse, error) {
	panic("AcquireSession is not used by this test")
}

func (r *testStateRouter) SaveOfflineState(context.Context, staterouter.SaveOfflineStateRequest) error {
	panic("SaveOfflineState is not used by this test")
}

func (r *testStateRouter) Subscribe(
	ctx context.Context,
	req staterouter.SubscribeRequest,
) (*staterouter.SubscribeResponse, error) {
	if r.subscribe == nil {
		panic("Subscribe is not configured")
	}
	return r.subscribe(ctx, req)
}

func (r *testStateRouter) DeleteClientSubscriptions(
	context.Context,
	staterouter.DeleteClientSubscriptionsRequest,
) error {
	panic("DeleteClientSubscriptions is not used by this test")
}

func (r *testStateRouter) ListClientSubscriptions(
	context.Context,
	staterouter.ListClientSubscriptionsRequest,
) (*staterouter.ListClientSubscriptionsResponse, error) {
	return &staterouter.ListClientSubscriptionsResponse{}, nil
}

func (r *testStateRouter) RoutePublish(ctx context.Context, req staterouter.RoutePublishRequest) error {
	if r.routePublish == nil {
		panic("RoutePublish is not configured")
	}
	return r.routePublish(ctx, req)
}

func (r *testStateRouter) HasMatchingSubscribers(
	ctx context.Context,
	req staterouter.HasMatchingSubscribersRequest,
) (bool, error) {
	if r.hasMatchingSubscribers == nil {
		panic("HasMatchingSubscribers is not configured")
	}
	return r.hasMatchingSubscribers(ctx, req)
}

func (r *testStateRouter) ClosePreviousOwner(context.Context, staterouter.ClosePreviousOwnerRequest) error {
	panic("ClosePreviousOwner is not used by this test")
}

var _ staterouter.Client = (*testStateRouter)(nil)

func newTestInProcessStateRouter(
	t *testing.T,
	sessionCenter session.Center,
	subscriptionCenter subscription.Center,
) staterouter.Client {
	t.Helper()
	router, err := staterouter.NewInProcessClient(staterouter.InProcessDependencies{
		SessionCenter:      sessionCenter,
		SubscriptionCenter: subscriptionCenter,
		PublishRoute: func(context.Context, staterouter.RoutePublishRequest) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new test state router: %v", err)
	}
	return router
}

func newTestPublishStateRouter(route func(context.Context, *brokerpublish.Message) error) staterouter.Client {
	return &testStateRouter{
		routePublish: func(ctx context.Context, req staterouter.RoutePublishRequest) error {
			return route(ctx, req.Message)
		},
		hasMatchingSubscribers: func(context.Context, staterouter.HasMatchingSubscribersRequest) (bool, error) {
			return true, nil
		},
	}
}
