package session

import (
	"encoding/json"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"time"
)

const (
	KeyClientPrefix               = "client/"
	KeyClientUnAckMessageID       = `/unack/message_id`
	keyClientUnfinishedMessage    = `/unfinished/message`
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
	store    *broker.SessionStoreWithTimeout
}

func newSession(clientID string, store broker.SessionStore) *Session {
	return &Session{
		clientID: clientID,
		store: broker.NewSessionStoreWithTimout(
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

func (s *Session) CreateTopicUnFinishedMessage(topic string, message []broker.UnFinishedMessage) {
	prefix := clientTopicUnFinishedMessagePrefix(s.clientID, topic)
	for _, m := range message {
		payload, err := json.Marshal(m)
		if err != nil {
			logger.Logger.Error("marshal unfinished message failed", zap.Error(err), zap.String("clientID", s.clientID))
			continue
		}
		if err := s.store.DefaultPutKey(prefix+"/"+m.MessageID, string(payload)); err != nil {
			logger.Logger.Error("create topic unfinished message failed", zap.Error(err), zap.String("clientID", s.clientID),
				zap.String("topic", topic), zap.String("messageID", m.MessageID))
		}
	}
}

func (s *Session) ReadTopicUnFinishedMessage(topic string) (message []broker.UnFinishedMessage) {
	prefix := clientTopicUnFinishedMessagePrefix(s.clientID, topic)

	m, err := s.store.DefaultReadPrefixKey(prefix)
	if err != nil {
		logger.Logger.Error("read topic unfinished message failed", zap.Error(err), zap.String("clientID", s.clientID))
		return
	}
	for _, v := range m {
		var unfinishedMessage broker.UnFinishedMessage
		if err := json.Unmarshal([]byte(v), &unfinishedMessage); err != nil {
			logger.Logger.Error("unmarshal unfinished message failed", zap.Error(err), zap.String("clientID", s.clientID))
			continue
		}
		message = append(message, unfinishedMessage)
	}
	return message
}

func (s *Session) DeleteTopicUnFinishedMessage(topic string, messageID string) {
	if err := s.store.DefaultDeleteKey(clientTopicUnFinishedMessagePrefix(s.clientID, topic)); err != nil {
		logger.Logger.Error("delete topic unfinished message failed", zap.Error(err), zap.String("clientID", s.clientID))
	}
}
