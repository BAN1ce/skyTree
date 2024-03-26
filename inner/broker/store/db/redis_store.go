package db

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

// TODO: implement

type Redis struct {
	db *redis.Client
}

func NewRedis() *Redis {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	return &Redis{db: rdb}

}

func (r *Redis) PutKey(ctx context.Context, key, value string) error {
	return r.db.Set(ctx, key, value, 0).Err()
}

func (r *Redis) ReadKey(ctx context.Context, key string) (string, bool, error) {
	if tmp, err := r.db.Get(ctx, key).Result(); err != nil {
		return "", false, err
	} else {
		return tmp, true, nil
	}

}

func (r *Redis) DeleteKey(ctx context.Context, key string) error {
	return r.db.Del(ctx, key).Err()
}

func (r *Redis) ReadPrefixKey(ctx context.Context, prefix string) (map[string]string, error) {
	var (
		keys   []string
		err    error
		tmp    []string
		cursor uint64
	)
	for {
		tmp, cursor, err = r.db.Scan(ctx, cursor, prefix+"*", 1000).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, tmp...)
		if cursor == 0 {
			break
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	values, err := r.db.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(keys))
	for i, value := range values {
		if valueStr, ok := value.(string); ok {
			result[keys[i]] = valueStr
		} else {
			return nil, fmt.Errorf("value for key %s is not a string", keys[i])
		}
	}
	return result, nil
}

func (r *Redis) DeletePrefixKey(ctx context.Context, prefix string) error {
	var (
		keys   []string
		err    error
		tmp    []string
		cursor uint64
	)
	for {
		result := r.db.Scan(ctx, cursor, prefix, 1000)
		tmp, cursor, err = result.Result()
		keys = append(keys, tmp...)
		if err != nil {
			break
		}
		if cursor == 0 {
			break
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return r.db.Del(ctx, keys...).Err()
}

func (r *Redis) ZAdd(ctx context.Context, key, value string, score float64) error {
	return r.db.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: value,
	}).Err()
}

func (r *Redis) ZDel(ctx context.Context, key, member string) error {
	return r.db.ZRem(ctx, key, member).Err()
}

func (r *Redis) ZRange(ctx context.Context, key string, start, end float64) ([]string, error) {
	return r.db.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", start),
		Max: fmt.Sprintf("%f", end),
	}).Result()
}
