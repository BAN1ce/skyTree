package redis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/config"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	redisv9 "github.com/redis/go-redis/v9"
)

// Redis implements store.KeyStoreWithBackup backed by Redis.
//
// Snapshot/Recover are not supported because Redis is an external service and
// this interface is primarily used by local/raft-based keystores.
type Redis struct {
	client *redisv9.Client
}

func NewRedis(cfg config.Redis) *Redis {
	opt := &redisv9.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.ConnectTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.MaxActive,
		MinIdleConns: cfg.MaxIdle,
	}
	return &Redis{client: redisv9.NewClient(opt)}
}

func (r *Redis) Close() error {
	return r.client.Close()
}

func (r *Redis) PutKey(ctx context.Context, key, value []byte) error {
	return r.client.Set(ctx, string(key), value, 0).Err()
}

func (r *Redis) ReadKey(ctx context.Context, key []byte) ([]byte, bool, error) {
	b, err := r.client.Get(ctx, string(key)).Bytes()
	if errors.Is(err, redisv9.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (r *Redis) DeleteKey(ctx context.Context, key []byte) error {
	return r.client.Del(ctx, string(key)).Err()
}

func (r *Redis) DeletePrefixKey(ctx context.Context, prefix []byte) error {
	p := string(prefix)
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, p+"*", 500).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (r *Redis) HSet(ctx context.Context, key []byte, field [][]byte) error {
	if len(field) == 0 {
		return nil
	}
	args := make([]interface{}, 0, len(field))
	for i := 0; i < len(field); i += 2 {
		if i+1 >= len(field) {
			break
		}
		args = append(args, string(field[i]), field[i+1])
	}
	return r.client.HSet(ctx, string(key), args...).Err()
}

func (r *Redis) HGet(ctx context.Context, key, field []byte) ([]byte, bool, error) {
	b, err := r.client.HGet(ctx, string(key), string(field)).Bytes()
	if errors.Is(err, redisv9.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (r *Redis) HDel(ctx context.Context, key []byte, field [][]byte) error {
	if len(field) == 0 {
		return nil
	}
	fields := make([]string, 0, len(field))
	for _, f := range field {
		fields = append(fields, string(f))
	}
	return r.client.HDel(ctx, string(key), fields...).Err()
}

func (r *Redis) HGetAll(ctx context.Context, key []byte) (map[string]string, error) {
	return r.client.HGetAll(ctx, string(key)).Result()
}

func (r *Redis) HPrefix(ctx context.Context, key []byte, prefix []byte) (map[string]string, error) {
	k := string(key)
	p := string(prefix)
	out := make(map[string]string)

	var cursor uint64
	for {
		items, next, err := r.client.HScan(ctx, k, cursor, p+"*", 500).Result()
		if err != nil {
			return nil, err
		}
		for i := 0; i+1 < len(items); i += 2 {
			out[items[i]] = items[i+1]
		}
		cursor = next
		if cursor == 0 {
			return out, nil
		}
	}
}

func (r *Redis) DeleteHash(ctx context.Context, key []byte) error {
	return r.client.Del(ctx, string(key)).Err()
}

func (r *Redis) SetExpired(ctx context.Context, key []byte, duration time.Duration) error {
	if duration <= 0 {
		return fmt.Errorf("set expired: duration must be > 0, got %s", duration)
	}
	_, err := r.client.Expire(ctx, string(key), duration).Result()
	return err
}

func (r *Redis) Snapshot(writer io.Writer) error {
	return fmt.Errorf("redis Snapshot is not supported")
}

func (r *Redis) Recover(reader io.Reader) error {
	// Drain reader to avoid surprising callers that stream snapshot data.
	// The data is intentionally ignored.
	_, _ = io.Copy(io.Discard, reader)
	return fmt.Errorf("redis Recover is not supported")
}

var _ store.KeyStoreWithBackup = (*Redis)(nil)

// normalizePrefix ensures DeletePrefixKey behavior stays consistent across callers.
// It is intentionally unused for now but kept for future compatibility.
func normalizePrefix(prefix []byte) []byte {
	return bytes.TrimRightFunc(prefix, func(r rune) bool {
		return strings.ContainsRune("\r\n\t ", r)
	})
}
