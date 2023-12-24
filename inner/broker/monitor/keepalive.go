package monitor

import (
	"context"
	"github.com/BAN1ce/skyTree/logger"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	ClientAliveTimeStoreKey = "client_alive_time"
)

type ZSet interface {
	ZAdd(ctx context.Context, key, member string, score float64) error
	ZDel(ctx context.Context, key, member string) error
	ZRange(ctx context.Context, key string, start, end float64) ([]string, error)
}

type OnClientExpired func(clientID []string)

type KeepAlive struct {
	ZSet
	tinker      *time.Ticker
	interval    time.Duration
	expiredFunc OnClientExpired
	cancel      context.CancelFunc
	mux         sync.RWMutex
}

func NewKeepAlive(store ZSet, interval time.Duration, expired OnClientExpired) *KeepAlive {
	return &KeepAlive{
		ZSet:        store,
		interval:    interval,
		expiredFunc: expired,
	}

}

func (k *KeepAlive) Start(ctx context.Context) error {
	ctx, k.cancel = context.WithCancel(ctx)
	k.tinker = time.NewTicker(k.interval)
	defer func() {
		k.tinker.Stop()
	}()
	for {
		select {
		case <-ctx.Done():
			logger.Logger.Info("keepalive monitor exit by context done")
			return nil
		case t, ok := <-k.tinker.C:
			if !ok {
				logger.Logger.Info("keepalive monitor exit by tinker close")
				return nil
			}
			k.mux.RLock()
			uid, err := k.ZRange(ctx, ClientAliveTimeStoreKey, 0, float64(t.Unix()))
			k.mux.RUnlock()
			if err != nil {
				logger.Logger.Warn("keepalive monitor error", zap.Error(err))
			}
			if k.expiredFunc != nil && len(uid) > 0 {
				logger.Logger.Debug("client keep alive expired, do expired function", zap.Strings("uid", uid))
				k.expiredFunc(uid)
			}
		}

	}
}

func (k *KeepAlive) SetClientAliveTime(uid string, t *time.Time) error {
	k.mux.Lock()
	defer k.mux.Unlock()
	var (
		ctx, cancel = context.WithTimeout(context.TODO(), 5*time.Second)
	)
	defer cancel()
	logger.Logger.Debug("set client alive time", zap.String("uid", uid), zap.String("time", t.String()))
	return k.ZAdd(ctx, ClientAliveTimeStoreKey, uid, float64(t.Unix()))
}

func (k *KeepAlive) Close() error {
	k.cancel()
	return nil
}

func (k *KeepAlive) DeleteClient(ctx context.Context, uid string) error {
	k.mux.Lock()
	defer k.mux.Unlock()
	return k.ZSet.ZDel(ctx, ClientAliveTimeStoreKey, uid)
}
