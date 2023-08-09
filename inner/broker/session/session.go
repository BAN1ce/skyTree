package session

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"time"
)

const (
	KeyClientPrefix               = "client/"
	KeyClientUnAckMessageID       = `/unack/message_id`
	KeyClientUnRecPacketID        = `/unrec/packet_id`
	KeyClientUnCompPacketID       = `/uncomp/packet_id`
	KeyClientLatestAliveTime      = `/latest_alive_time`
	KeyClientLatestAckedMessageID = `/latest_acked_message_id`
	KeyClientSubTopic             = `/sub_topic`
)

func clientKey(clientID string) *strings.Builder {
	var build = strings.Builder{}
	build.WriteString(KeyClientPrefix)
	build.WriteString(clientID)
	return &build

}
func withClientKey(key, clientID string) string {
	build := clientKey(clientID)
	build.WriteString("/")
	build.WriteString(key)
	return build.String()
}

type Session struct {
	clientID string
	store    *pkg.SessionStoreWithTimeout
}

func newSession(clientID string, store pkg.SessionStore) *Session {
	return &Session{
		clientID: clientID,
		store: pkg.NewSessionStoreWithTimout(
			store,
			3*time.Second),
	}
}

func (s *Session) Release() {
	if err := s.store.DefaultDeleteKey(clientKey(s.clientID).String()); err != nil {
		logger.Logger.Error("release session failed", zap.Error(err), zap.String("clientID", s.clientID))
	}
}

func (s *Session) ReadSubTopics() (topics map[string]int32) {
	topics = make(map[string]int32)
	m, err := s.store.DefaultReadPrefixKey(clientSubTopicKeyPrefix(s.clientID))
	if err != nil {
		logger.Logger.Error("read sub topics failed", zap.Error(err), zap.String("clientID", s.clientID))
		return nil
	}
	for k, v := range m {
		// QoS should not greater than 2, so int is enough
		qos, err := strconv.Atoi(v)
		if err != nil {
			logger.Logger.Error("string to int32 failed", zap.Error(err), zap.String("clientID", s.clientID))
			continue
		}
		if topic := strings.TrimPrefix(k, clientSubTopicKeyPrefix(s.clientID)); topic == "" {
			logger.Logger.Error("trim prefix failed", zap.Error(err), zap.String("clientID", s.clientID))
			continue
		} else {
			topics[topic] = int32(qos)
		}
	}
	return
}

func (s *Session) CreateSubTopic(topic string, qos int32) {
	if topic == "" {
		return
	}
	if err := s.store.DefaultPutKey(clientSubTopicKey(s.clientID, topic), strconv.Itoa(int(qos))); err != nil {
		logger.Logger.Error("create sub topic failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.Int32("qos", qos))
	}
}

func (s *Session) DeleteSubTopic(topic string) {
	if topic == "" {
		return
	}
	if err := s.store.DefaultDeleteKey(clientSubTopicKey(s.clientID, topic)); err != nil {
		logger.Logger.Error("delete sub topic failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic))
	}
}

func (s *Session) ReadTopicUnAckMessageID(topic string) (id []string) {
	if topic == "" {
		return
	}
	m, err := s.store.DefaultReadPrefixKey(clientTopicUnAckKeyPrefix(s.clientID, topic))
	if err != nil {
		logger.Logger.Error("read topic unack message id failed", zap.Error(err), zap.String("clientID", s.clientID))
		return nil
	}
	for _, v := range m {
		id = append(id, v)
	}
	return
}

func (s *Session) CreateTopicUnAckMessageID(topic string, messageID []string) {
	if topic == "" {
		return
	}
	for _, id := range messageID {
		if err := s.store.DefaultPutKey(clientTopicUnAckKey(s.clientID, topic, id), id); err != nil {
			logger.Logger.Error("create topic unack message id failed", zap.Error(err), zap.String("clientID", s.clientID),
				zap.String("topic", topic), zap.String("messageID", id))
		}
	}
}

func (s *Session) DeleteTopicUnAckMessageID(topic string, messageID string) {
	if topic == "" {
		return
	}
	if err := s.store.DefaultDeleteKey(clientTopicUnAckKey(s.clientID, topic, messageID)); err != nil {
		logger.Logger.Error("delete topic unack message id failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.String("messageID", messageID))
	}
}

func (s *Session) ReadTopicUnRecPacketID(topic string) (packetID []string) {
	id, err := s.store.DefaultReadPrefixKey(withClientKey(KeyClientUnRecPacketID, s.clientID))
	if err != nil {
		logger.Logger.Error("read topic unrec packet id failed", zap.Error(err), zap.String("clientID", s.clientID))
		return nil
	}
	for _, v := range id {
		packetID = append(packetID, v)
	}
	return
}

func (s *Session) CreateTopicUnRecPacketID(topic string, packetID []string) {
	for _, id := range packetID {
		if err := s.store.DefaultPutKey(withClientKey(KeyClientUnRecPacketID, s.clientID)+"/"+id, ""); err != nil {
			logger.Logger.Error("create topic unrec packet id failed", zap.Error(err), zap.String("clientID", s.clientID),
				zap.String("topic", topic), zap.String("packetID", id))
		}
	}
}

func (s *Session) DeleteTopicUnRecPacketID(topic string, packetID string) {
	if err := s.store.DefaultDeleteKey(withClientKey(KeyClientUnRecPacketID, s.clientID) + "/" + packetID); err != nil {
		logger.Logger.Error("delete topic unrec packet id failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.String("packetID", packetID))
	}
}

func (s *Session) ReadTopicUnCompPacketID(topic string) (packetID []string) {
	id, err := s.store.DefaultReadPrefixKey(withClientKey(KeyClientUnCompPacketID, s.clientID))
	if err != nil {
		logger.Logger.Error("read topic uncomp packet id failed", zap.Error(err), zap.String("clientID", s.clientID))
		return nil
	}
	for _, v := range id {
		packetID = append(packetID, v)
	}
	return
}

func (s *Session) CreateTopicUnCompPacketID(topic string, packetID []string) {
	for _, id := range packetID {
		if err := s.store.DefaultPutKey(withClientKey(KeyClientUnCompPacketID, s.clientID)+"/"+id, ""); err != nil {
			logger.Logger.Error("create topic uncomp packet id failed", zap.Error(err), zap.String("clientID", s.clientID),
				zap.String("topic", topic), zap.String("packetID", id))
		}
	}
}

func (s *Session) DeleteTopicUnCompPacketID(topic string, packetID string) {
	if err := s.store.DefaultDeleteKey(withClientKey(KeyClientUnCompPacketID, s.clientID) + "/" + packetID); err != nil {
		logger.Logger.Error("delete topic uncomp packet id failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.String("packetID", packetID))
	}
}

func (s *Session) ReadTopicLatestPushedMessageID(topic string) (messageID string, ok bool) {
	id, _, err := s.store.DefaultReadKey(withClientKey(KeyClientLatestAckedMessageID, s.clientID))
	if err != nil {
		logger.Logger.Error("read topic last acked message id failed", zap.Error(err), zap.String("clientID", s.clientID))
		return "", false
	}
	return id, true
}

func (s *Session) SetTopicLatestPushedMessageID(topic string, messageID string) {
	if err := s.store.DefaultPutKey(withClientKey(KeyClientLatestAckedMessageID, s.clientID), messageID); err != nil {
		logger.Logger.Error("set topic last acked message id failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.String("messageID", messageID))
	}
}

func (s *Session) DeleteTopicLatestPushedMessageID(topic string, messageID string) {
	if err := s.store.DefaultDeleteKey(withClientKey(KeyClientLatestAckedMessageID, s.clientID)); err != nil {
		logger.Logger.Error("delete topic last acked message id failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.String("messageID", messageID))
	}
}
