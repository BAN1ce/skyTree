package store

import (
	"context"
	"errors"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/google/uuid"
	"github.com/nutsdb/nutsdb"
	"github.com/nutsdb/nutsdb/ds/zset"
	"go.uber.org/zap"
	"log"
	"math"
	"time"
)

type NutsDB struct {
	db *nutsdb.DB
}

func NewNutsDBStore() *NutsDB {
	var (
		err   error
		store = new(NutsDB)
	)
	store.db, err = nutsdb.Open(
		nutsdb.DefaultOptions,
		// TODO: support config
		nutsdb.WithDir("./data/nutsdb"),
	)
	if err != nil {
		log.Fatalln("open db error: ", err)
	}
	return store
}

func (s *NutsDB) CreatePacket(topic string, value []byte) (id string, err error) {
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

func (s *NutsDB) ReadFromTimestamp(ctx context.Context, topic string, timestamp time.Time, limit int) ([]packet.Publish, error) {
	var (
		messages []packet.Publish
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

func (s *NutsDB) ReadTopicMessagesByID(ctx context.Context, topic, id string, limit int, include bool) ([]packet.Publish, error) {
	var (
		messages []packet.Publish
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

func (s *NutsDB) DeleteBeforeID(id string) {
	// TODO implement me
	panic("implement me")
}

func nutsDBValuesBeMessages(values []*zset.SortedSetNode, topic string) []packet.Publish {
	var (
		messages []packet.Publish
	)
	for _, v := range values {
		if pubPacket, err := pkg.Decode(v.Value); err != nil {
			logger.Logger.Error("read from NutsDB decode error: ", zap.Error(err))
			continue
		} else {
			messages = append(messages, packet.Publish{
				MessageID: v.Key(),
				Packet:    pubPacket,
			})
		}
	}
	return messages
}
