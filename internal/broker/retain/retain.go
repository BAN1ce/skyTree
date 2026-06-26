package retain

import (
	"context"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
	"github.com/BAN1ce/skyTree/proto/proto_retain"
	"google.golang.org/protobuf/proto"
)

type Store struct {
	store *store.HashStoreWithTimeout

	gcMu     sync.Mutex
	gcCancel context.CancelFunc
}

type GCPredicate func(context.Context) bool

func NewRetainStore(hashStore store.HashStore) *Store {
	return NewRetainStoreWithContext(context.Background(), hashStore)
}

func NewRetainStoreWithContext(ctx context.Context, hashStore store.HashStore) *Store {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Store{
		store: store.NewHashStoreWithTimeoutFromContext(ctx, hashStore, 5*time.Second),
	}
}

func (d *Store) PutRetainMessage(message *proto_retain.RetainMessage) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	logger.Logger.Debug().Str("topic", message.Topic).Str("data", string(data)).Msg("put retain message")
	return d.store.DefaultHSet(GetTopicRetainKey(), [][]byte{[]byte(message.Topic), data})
}

func (d *Store) GetRetainMessage(topic string) (*proto_retain.RetainMessage, bool) {
	data, ok, err := d.store.DefaultHGet(GetTopicRetainKey(), []byte(topic))
	if err != nil && !store.IsNotFound(err) {
		logger.Logger.Error().Err(err).Str("topic", topic).Msg("get retain message failed")
		return nil, false
	}

	if !ok {
		return nil, false
	}

	var message proto_retain.RetainMessage

	if err := proto.Unmarshal(data, &message); err != nil {
		logger.Logger.Error().Err(err).Str("topic", topic).Msg("unmarshal retain message failed")
		return nil, false
	}

	// Lazy 清理：读时若已过期，顺手删一条。
	if d.isExpired(&message, time.Now()) {
		if err := d.DeleteRetainMessage(message.Topic); err != nil {
			logger.Logger.Warn().Err(err).Str("topic", message.Topic).Msg("delete expired retain message failed")
		}
		return nil, false
	}

	return &message, true
}

func (d *Store) DeleteRetainMessage(topic string) error {
	return d.store.DefaultHDel(GetTopicRetainKey(), [][]byte{[]byte(topic)})

}

func (d *Store) GetRetainMessagesByTopicFilter(topicFilter string) ([]*proto_retain.RetainMessage, error) {
	all, err := d.store.DefaultHGetAll(GetTopicRetainKey())
	if err != nil {
		return nil, err
	}

	now := time.Now()
	messages := make([]*proto_retain.RetainMessage, 0)
	var expiredTopics [][]byte
	for topic, data := range all {
		if !topicutil.MatchTopicFilter(topicFilter, topic) {
			continue
		}
		var message proto_retain.RetainMessage
		if err := proto.Unmarshal([]byte(data), &message); err != nil {
			logger.Logger.Error().Err(err).Str("topic", topic).Msg("unmarshal retain message failed")
			continue
		}
		if d.isExpired(&message, now) {
			expiredTopics = append(expiredTopics, []byte(topic))
			continue
		}
		messages = append(messages, &message)
	}
	if len(expiredTopics) > 0 {
		if err := d.store.DefaultHDel(GetTopicRetainKey(), expiredTopics); err != nil {
			logger.Logger.Warn().Err(err).Int("count", len(expiredTopics)).Msg("lazy delete expired retain messages failed")
		}
	}
	return messages, nil
}

// StartGC 启动后台清理已过期 retained 消息的 goroutine。interval<=0 时直接返回。
// 调用方在 broker 关停时应调用 StopGC 释放 goroutine。
func (d *Store) StartGC(ctx context.Context, interval time.Duration) {
	d.StartGCWithPredicate(ctx, interval, nil)
}

// StartGCWithPredicate starts retained message GC and skips a cycle when shouldRun returns false.
func (d *Store) StartGCWithPredicate(ctx context.Context, interval time.Duration, shouldRun GCPredicate) {
	if d == nil || interval <= 0 {
		return
	}
	d.gcMu.Lock()
	if d.gcCancel != nil {
		d.gcMu.Unlock()
		return
	}
	gcCtx, cancel := context.WithCancel(ctx)
	d.gcCancel = cancel
	d.gcMu.Unlock()

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-gcCtx.Done():
				return
			case <-t.C:
				if shouldRun != nil && !shouldRun(gcCtx) {
					continue
				}
				if removed, err := d.RunGCOnce(gcCtx); err != nil {
					logger.Logger.Warn().Err(err).Msg("retain gc cycle failed")
				} else if removed > 0 {
					logger.Logger.Info().Int("removed", removed).Msg("retain gc cycle removed expired messages")
				}
			}
		}
	}()
}

// StopGC 停止后台清理。
func (d *Store) StopGC() {
	if d == nil {
		return
	}
	d.gcMu.Lock()
	defer d.gcMu.Unlock()
	if d.gcCancel != nil {
		d.gcCancel()
		d.gcCancel = nil
	}
}

// RunGCOnce 同步扫描并清理已过期的 retained 消息，返回被清理的条数。
// 主要供单测以及人工触发的场景使用。
func (d *Store) RunGCOnce(ctx context.Context) (int, error) {
	if d == nil {
		return 0, nil
	}
	all, err := d.store.DefaultHGetAll(GetTopicRetainKey())
	if err != nil {
		return 0, err
	}
	now := time.Now()
	var expiredTopics [][]byte
	for topic, data := range all {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		var message proto_retain.RetainMessage
		if err := proto.Unmarshal([]byte(data), &message); err != nil {
			// 解码失败的脏数据也清掉，避免一直阻塞 GC。
			expiredTopics = append(expiredTopics, []byte(topic))
			continue
		}
		if d.isExpired(&message, now) {
			expiredTopics = append(expiredTopics, []byte(topic))
		}
	}
	if len(expiredTopics) == 0 {
		return 0, nil
	}
	if err := d.store.DefaultHDel(GetTopicRetainKey(), expiredTopics); err != nil {
		return 0, err
	}
	return len(expiredTopics), nil
}

// isExpired 判断 retained 消息是否已过期。
func (d *Store) isExpired(m *proto_retain.RetainMessage, now time.Time) bool {
	if m == nil {
		return false
	}
	if expiredAt := m.GetExpiredAtUnixNano(); expiredAt > 0 {
		return !now.Before(time.Unix(0, expiredAt))
	}
	return false
}

func GetTopicRetainKey() []byte {
	return store.KeyNamespaceRetain.KeyBytes()
}
