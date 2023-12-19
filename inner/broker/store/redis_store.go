package store

import (
	"context"
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
		result map[string]string
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
		return nil, nil
	}
	if value, err := r.db.MGet(ctx, keys...).Result(); err != nil {
		return result, err
	} else {
		for i := 0; i < len(value); i++ {
			if i < len(keys) {
				break
			}
			result[keys[i]] = value[i].(string)
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
