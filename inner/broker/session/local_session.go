package session

import (
	"errors"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/util"
	"github.com/nutsdb/nutsdb"
	"go.uber.org/zap"
	"math"
	"strings"
)

const (
	sessionBucket  = "client.proto"
	PrefixClient   = "client"
	PrefixSubTopic = "sub_topic."
	PrefixUnAck    = "un_ack."
	PrefixUnRec    = "un_rec"
	PrefixLastAck  = "last_ack"
)

type bucketID string

func (b *bucketID) clientSessionID() string {
	return PrefixClient + "." + string(*b)
}

func getSubTopicKey(topic string) string {
	return PrefixSubTopic + "." + topic
}

func getTopicUnAckKey(topic string) string {
	return PrefixUnAck + "." + topic
}

func getTopicUnAckMessageIDKey(topic, id string) string {
	return PrefixUnAck + topic + "." + id

}

func getTopicLastAckKey(topic string) string {
	return PrefixLastAck + "." + topic
}

type LocalSession struct {
	clientID bucketID
	db       *nutsdb.DB
}

func NewLocalSession(db *nutsdb.DB, clientID string) *LocalSession {
	return &LocalSession{
		clientID: bucketID(clientID),
		db:       db,
	}
}

func (l *LocalSession) Release() error {
	return l.db.Update(func(tx *nutsdb.Tx) error {
		var errs error
		err := tx.DeleteBucket(nutsdb.DataStructureBPTree, l.clientID.clientSessionID())
		if err != nil {
			errs = errors.Join(errs, err)
			logger.Logger.Error("delete bucket error", zap.Error(err))
		}
		err = tx.DeleteBucket(nutsdb.DataStructureSet, l.clientID.clientSessionID())
		if err != nil {
			errs = errors.Join(errs, err)
			logger.Logger.Error("delete bucket error", zap.Error(err))
		}
		err = tx.DeleteBucket(nutsdb.DataStructureList, l.clientID.clientSessionID())
		if err != nil {
			errs = errors.Join(errs, err)
			logger.Logger.Error("delete bucket error", zap.Error(err))
		}
		err = tx.DeleteBucket(nutsdb.DataStructureSortedSet, l.clientID.clientSessionID())
		if err != nil {
			logger.Logger.Error("delete bucket error", zap.Error(err))
		}
		return errs
	})
}

func (l *LocalSession) CreateSubTopic(topic string, qos int32) {
	err := l.db.Update(func(tx *nutsdb.Tx) error {
		return tx.Put(l.clientID.clientSessionID(), []byte(getSubTopicKey(topic)), util.Int32ToByte(qos), 0)
	})
	if err != nil {
		logger.Logger.Error("create sub topic error", zap.Error(err))
	}
}

func (l *LocalSession) ReadSubTopics() map[string]int32 {
	var (
		subTopics = make(map[string]int32)
	)
	err := l.db.View(func(tx *nutsdb.Tx) error {
		keys, _, err := tx.PrefixScan(l.clientID.clientSessionID(), []byte(PrefixSubTopic), 0, math.MaxInt)
		// if no sub topics
		if errors.Is(err, nutsdb.ErrPrefixScan) {
			err = nil
		}
		if err != nil {
			return err
		}
		for _, key := range keys {
			topic := strings.Trim(string(key.Key), PrefixSubTopic)
			topic = strings.Trim(topic, ".")
			subTopics[topic] = util.ByteToInt32(key.Value)
		}
		return err

	})
	if err != nil {
		logger.Logger.Error("read sub topics error", zap.Error(err))
	}
	return subTopics
}

func (l *LocalSession) CreateTopicUnAckMessageID(topic string, messageID []string) {
	err := l.db.Update(func(tx *nutsdb.Tx) error {
		for _, id := range messageID {
			if err := tx.Put(l.clientID.clientSessionID(), []byte(getTopicUnAckMessageIDKey(topic, id)), []byte(id), 0); err != nil {
				logger.Logger.Error("save topic un ack message id error", zap.Error(err), zap.String("topic", topic), zap.String("messageID", id))
			}
		}
		return nil
	})
	if err != nil {
		logger.Logger.Error("save topic un ack message id error", zap.Error(err))
	}
}

func (l *LocalSession) ReadTopicUnAckMessageID(topic string) []string {
	var id []string
	err := l.db.View(func(tx *nutsdb.Tx) error {
		keys, _, err := tx.PrefixScan(l.clientID.clientSessionID(), []byte(getTopicUnAckKey(topic)), 0, math.MaxInt)
		if err != nil {
			return err
		}
		for _, key := range keys {
			id = append(id, string(key.Value))
		}
		return nil

	})
	if err != nil {
		logger.Logger.Error("read topic un ack message id error", zap.Error(err))
	}
	return id
}
func (l *LocalSession) DeleteTopicUnAckMessageID(topic string, messageID string) {
	err := l.db.Update(func(tx *nutsdb.Tx) error {
		return tx.Delete(l.clientID.clientSessionID(), []byte(getTopicUnAckMessageIDKey(topic, messageID)))
	})
	if err != nil {
		logger.Logger.Error("delete topic un ack message id error", zap.Error(err))
	}
}

func (l *LocalSession) CreateTopicUnRecPacketID(topic string, packetID []string) {
	// TODO: implement me

}

func (l *LocalSession) UpdateTopicLastAckedMessageID(topic string, messageID string) {
	err := l.db.Update(func(tx *nutsdb.Tx) error {
		return tx.Put(l.clientID.clientSessionID(), []byte(PrefixLastAck+"."+topic), []byte(messageID), 0)
	})
	if err != nil {
		logger.Logger.Error("update topic last acked message id error", zap.Error(err))
	}
}

func (l *LocalSession) ReadTopicLastAckedMessageID(topic string) (string, bool) {
	var (
		key string
		ok  bool
	)
	err := l.db.View(func(tx *nutsdb.Tx) error {
		tmp, err := tx.Get(l.clientID.clientSessionID(), []byte(getTopicLastAckKey(topic)))
		if err != nil {
			return err
		}
		key = string(tmp.Value)
		return nil
	})
	if err != nil {
		logger.Logger.Error("read topic last acked message id error", zap.Error(err))
		return key, ok
	}
	ok = key != ""
	return key, ok
}

func (l *LocalSession) DeleteSubTopic(topic string) {
	err := l.db.Update(func(tx *nutsdb.Tx) error {
		return tx.Delete(l.clientID.clientSessionID(), []byte(getSubTopicKey(topic)))
	})
	if err != nil {
		logger.Logger.Error("delete sub topic error", zap.Error(err))
	}
}

func (l *LocalSession) ReadTopicUnRecPacketID(topic string) []string {
	// TODO implement me
	panic("implement me")
}

func (l *LocalSession) DeleteTopicUnRecPacketID(topic string, packetID string) {
	// TODO implement me
	panic("implement me")
}

func (l *LocalSession) ReadTopicUnCompPacketID(topic string) []string {
	// TODO implement me
	panic("implement me")
}

func (l *LocalSession) CreateTopicUnCompPacketID(topic string, packetID []string) {
	// TODO implement me
	panic("implement me")
}

func (l *LocalSession) DeleteTopicUnCompPacketID(topic string, packetID string) {
	// TODO implement me
	panic("implement me")
}

func (l *LocalSession) SetTopicLastAckedMessageID(topic string, messageID string) {
	// TODO implement me
	panic("implement me")
}

func (l *LocalSession) DeleteTopicLastAckedMessageID(topic string, messageID string) {
	// TODO implement me
	panic("implement me")
}
