package db

import (
	"context"
	"errors"
	"fmt"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/google/uuid"
	"github.com/nutsdb/nutsdb"
	"github.com/nutsdb/nutsdb/ds/zset"
	"go.uber.org/zap"
	"log"
	"math"
	"time"
)

var (
	nutsDB *nutsdb.DB
)

func initLocalDB(options nutsdb.Options, option ...nutsdb.Option) {
	var (
		err error
	)
	nutsDB, err = nutsdb.Open(
		// nutsdb.DefaultOptions,
		// TODO: support config
		// nutsdb.WithDir("./data/nutsdb"),
		options,
		option...,
	)
	if err != nil {
		log.Fatalln("open db error: ", err)
	}
}

const kvBucketName = "kvBucket"

type Local struct {
	db       *nutsdb.DB
	kvBucket string
}

func NewLocalStore(options nutsdb.Options, option ...nutsdb.Option) *Local {
	var (
		store = new(Local)
	)
	store.kvBucket = kvBucketName
	initLocalDB(options, option...)
	store.db = nutsDB
	if store.db == nil {
		logger.Logger.Panic("local db is nil")
	}
	return store
}

func (s *Local) CreatePacket(topic string, value []byte) (id string, err error) {
	var (
		timestamp = time.Now().UnixNano()
	)
	id = uuid.NewString()
	if err = s.db.Update(func(tx *nutsdb.Tx) error {
		return tx.ZAdd(topic, []byte(id), float64(timestamp), value)
	}); err != nil {
		return
	}
	return
}

func (s *Local) ReadFromTimestamp(ctx context.Context, topic string, timestamp time.Time, limit int) ([]*packet.Message, error) {
	var (
		messages []*packet.Message
		err      error
	)
	err = s.db.View(func(tx *nutsdb.Tx) error {
		tmp, err := tx.ZRangeByScore(topic, float64(timestamp.UnixNano()), math.MaxFloat64, &zset.GetByScoreRangeOptions{
			Limit: limit,
		})
		if err != nil {
			return err
		}
		messages = nutsDBValuesBeMessages(tmp, topic)
		return nil
	})
	return messages, err

}

func (s *Local) ReadTopicMessagesByID(ctx context.Context, topic, id string, limit int, include bool) ([]*packet.Message, error) {
	var (
		messages []*packet.Message
		err      error
	)
	err = s.db.View(func(tx *nutsdb.Tx) error {
		score, err := tx.ZScore(topic, []byte(id))
		if err != nil && !errors.Is(err, nutsdb.ErrKeyNotFound) {
			return err
		}

		if score == 0 {
			tmp, err := tx.ZPeekMax(topic)
			if err != nil {
				return err
			} else {
				score = float64(tmp.Score())
			}
		}
		if tmp, err := tx.ZRangeByScore(topic, score, math.MaxFloat64, &zset.GetByScoreRangeOptions{
			Limit: limit,
		}); err != nil {
			return err
		} else {
			if !include {
				if len(tmp) > 0 {
					tmp = tmp[1:]
				}
			}
			messages = nutsDBValuesBeMessages(tmp, topic)
			return nil
		}
	})
	return messages, err
}

func (s *Local) DeleteTopicMessageID(ctx context.Context, topic, messageID string) error {
	return s.db.Update(func(tx *nutsdb.Tx) error {
		return tx.ZRem(topic, messageID)
	})
}
func (s *Local) DeleteBeforeID(id string) {
	// TODO implement me
	panic("implement me")
}

func nutsDBValuesBeMessages(values []*zset.SortedSetNode, topic string) []*packet.Message {
	var (
		messages []*packet.Message
	)
	for _, v := range values {
		if pubMessage, err := broker.Decode(v.Value); err != nil {
			logger.Logger.Error("read from Local decode error: ", zap.Error(err))
			continue
		} else {
			if pubMessage.ExpiredTime == 0 || pubMessage.ExpiredTime > time.Now().Unix() {
				pubMessage.MessageID = v.Key()
				messages = append(messages, pubMessage)
			} else {
				logger.Logger.Debug("read from Local decode message expired", zap.String("topic", topic), zap.String("messageID", pubMessage.MessageID))
			}
		}
	}
	return messages
}

//
// Key Value MessageStore
//

func (s *Local) PutKey(ctx context.Context, key, value string) error {
	var err error
	logger.Logger.Debug("store put key", zap.String("key", key), zap.String("value", value))
	if err = s.db.Update(func(tx *nutsdb.Tx) error {
		return tx.SAdd(s.kvBucket, []byte(key), []byte(value))
	}); err != nil {
		return err
	}
	return nil
}

func (s *Local) ReadKey(ctx context.Context, key string) (string, bool, error) {
	var (
		value string
		err   error
		ok    bool
	)
	logger.Logger.Debug("store read key", zap.String("key", key))
	if err = s.db.View(func(tx *nutsdb.Tx) error {
		if v, err := tx.SMembers(s.kvBucket, []byte(key)); err != nil {
			return err
		} else {
			if len(v) > 0 {
				ok = true
				value = string(v[0])
			}
			return nil
		}
	}); err != nil {
		return "", false, err
	}
	return value, ok, nil
}

func (s *Local) DeleteKey(ctx context.Context, key string) error {
	logger.Logger.Debug("store delete key", zap.String("key", key))
	if err := s.db.Update(func(tx *nutsdb.Tx) error {
		return tx.SRem(s.kvBucket, []byte(key))
	}); err != nil {
		return err
	}
	return nil
}

func (s *Local) ReadPrefixKey(ctx context.Context, prefix string) (map[string]string, error) {
	var (
		values = make(map[string]string)
		err    error
	)
	defer func() {
		logger.Logger.Debug("store read prefix key", zap.String("key", prefix), zap.Any("values", values), zap.String("ctx", pkg.GetContextID(ctx)))
	}()
	if err = s.db.View(func(tx *nutsdb.Tx) error {
		// TODO: limit number set 9999, it's dangerous
		if entries, _, err := tx.PrefixScan(s.kvBucket, []byte(prefix), 0, 9999); err != nil {
			return err
		} else {
			for _, entry := range entries {
				values[string(entry.Key)] = string(entry.Value)
			}
			return nil
		}
	}); err != nil {
		if errors.Is(err, nutsdb.ErrPrefixScan) {
			return nil, errors.Join(errs.ErrStoreKeyNotFound, fmt.Errorf("prefix key: %s", prefix))
		}
		return nil, err
	}
	return values, nil
}

func (s *Local) DeletePrefixKey(ctx context.Context, prefix string) error {
	m, err := s.ReadPrefixKey(ctx, prefix)
	if err != nil {
		return err
	}

	for k := range m {
		if tmp := s.DeleteKey(ctx, k); err != nil {
			err = errors.Join(err, tmp)
		}
	}
	return err
}

func (s *Local) ZAdd(ctx context.Context, key, member string, score float64) error {
	return s.db.Update(func(tx *nutsdb.Tx) error {
		return tx.ZAdd(s.kvBucket, []byte(key), score, []byte(member))
	})
}

func (s *Local) ZDel(ctx context.Context, key, member string) error {
	return s.db.Update(func(tx *nutsdb.Tx) error {
		return tx.ZRem(s.kvBucket, key)
	})
}

func (s *Local) ZRange(ctx context.Context, key string, start, end float64) ([]string, error) {
	var (
		result []string
	)
	err := s.db.View(func(tx *nutsdb.Tx) error {
		tmp, err := tx.ZRangeByScore(s.kvBucket, start, end, nil)
		if err != nil {
			return err
		}
		for _, v := range tmp {
			result = append(result, v.Key())
		}
		return nil
	})
	return result, err
}
