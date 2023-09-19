package session

import (
	"encoding/json"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/errs"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"time"
)

type Session struct {
	clientID string
	store    *broker.KeyValueStoreWithTimeout
}

func newSession(clientID string, store broker.KeyValueStore) *Session {
	return &Session{
		clientID: clientID,
		store: broker.NewKeyValueStoreWithTimout(
			store,
			3*time.Second),
	}
}

func (s *Session) Release() {
	if err := s.store.DefaultDeleteKey(broker.ClientKey(s.clientID).String()); err != nil {
		logger.Logger.Error("release session failed", zap.Error(err), zap.String("clientID", s.clientID))
	}
}

func (s *Session) ReadSubTopics() (topics map[string]int32) {
	topics = make(map[string]int32)
	m, err := s.store.DefaultReadPrefixKey(broker.ClientSubTopicKeyPrefix(s.clientID))
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
		if topic := strings.TrimPrefix(k, broker.ClientSubTopicKeyPrefix(s.clientID)); topic == "" {
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
	if err := s.store.DefaultPutKey(broker.ClientSubTopicKey(s.clientID, topic), strconv.Itoa(int(qos))); err != nil {
		logger.Logger.Error("create sub topic failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.Int32("qos", qos))
	}
}

func (s *Session) DeleteSubTopic(topic string) {
	if topic == "" {
		return
	}
	if err := s.store.DefaultDeleteKey(broker.ClientSubTopicKey(s.clientID, topic)); err != nil {
		logger.Logger.Error("delete sub topic failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic))
	}
}

func (s *Session) ReadTopicLatestPushedMessageID(topic string) (messageID string, ok bool) {
	id, _, err := s.store.DefaultReadKey(broker.WithClientKey(broker.KeyClientLatestAckedMessageID, s.clientID))
	if err != nil {
		logger.Logger.Error("read topic last acked message id failed", zap.Error(err), zap.String("clientID", s.clientID))
		return "", false
	}
	return id, true
}

func (s *Session) SetTopicLatestPushedMessageID(topic string, messageID string) {
	if err := s.store.DefaultPutKey(broker.WithClientKey(broker.KeyClientLatestAckedMessageID, s.clientID), messageID); err != nil {
		logger.Logger.Error("set topic last acked message id failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.String("messageID", messageID))
	}
}

func (s *Session) DeleteTopicLatestPushedMessageID(topic string, messageID string) {
	if err := s.store.DefaultDeleteKey(broker.WithClientKey(broker.KeyClientLatestAckedMessageID, s.clientID)); err != nil {
		logger.Logger.Error("delete topic last acked message id failed", zap.Error(err), zap.String("clientID", s.clientID),
			zap.String("topic", topic), zap.String("messageID", messageID))
	}
}

func (s *Session) CreateTopicUnFinishedMessage(topic string, message []broker.UnFinishedMessage) {
	prefix := broker.ClientTopicUnFinishedMessagePrefix(s.clientID, topic)
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
	prefix := broker.ClientTopicUnFinishedMessagePrefix(s.clientID, topic)

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
	if err := s.store.DefaultDeleteKey(broker.ClientTopicUnFinishedMessagePrefix(s.clientID, topic)); err != nil {
		logger.Logger.Error("delete topic unfinished message failed", zap.Error(err), zap.String("clientID", s.clientID))
	}
}

func (s *Session) GetWillMessage() (*broker.WillMessage, error) {
	var message broker.WillMessage
	if value, ok, _ := s.store.DefaultReadKey(broker.ClientWillMessageKey(s.clientID)); ok {
		if err := json.Unmarshal([]byte(value), &message); err != nil {
			logger.Logger.Error("unmarshal will message failed", zap.Error(err), zap.String("clientID", s.clientID))
			return &message, err
		}
		return &message, nil

	}
	return nil, errs.ErrSessionWillMessageNotFound

}

func (s *Session) SetWillMessage(message *broker.WillMessage) error {
	if jBody, err := json.Marshal(message); err != nil {
		return err
	} else {
		return s.store.DefaultPutKey(broker.ClientWillMessageKey(s.clientID), string(jBody))
	}
}

func (s *Session) GetConnectProperties() (*broker.SessionConnectProperties, error) {
	var (
		properties broker.SessionConnectProperties
	)
	value, ok, err := s.store.DefaultReadKey(broker.ClientConnectPropertiesKey(s.clientID))
	if err != nil {
		logger.Logger.Error("read connect properties failed", zap.Error(err), zap.String("clientID", s.clientID))
		return &properties, err
	}
	if !ok {
		return &properties, errs.ErrSessionConnectPropertiesNotFound
	}
	if err := json.Unmarshal([]byte(value), &properties); err != nil {
		logger.Logger.Error("unmarshal connect properties failed", zap.Error(err), zap.String("clientID", s.clientID))
		return &properties, err
	}
	return &properties, nil
}

func (s *Session) SetConnectProperties(properties *broker.SessionConnectProperties) error {
	value, err := json.Marshal(properties)
	if err != nil {
		logger.Logger.Error("marshal connect properties failed", zap.Error(err), zap.String("clientID", s.clientID))
		return err
	}
	if err := s.store.DefaultPutKey(broker.ClientConnectPropertiesKey(s.clientID), string(value)); err != nil {
		logger.Logger.Error("set connect properties failed", zap.Error(err), zap.String("clientID", s.clientID))
		return err
	}
	return nil
}
