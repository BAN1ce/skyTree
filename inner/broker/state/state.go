package state

import (
	"context"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/store"
	"time"
)

type State struct {
	store *store.KeyValueStoreWithTimeout
}

func NewState(keyStore store.KeyStore) *State {
	return &State{
		store: store.NewKeyValueStoreWithTimout(keyStore, 3*time.Second),
	}
}

func (s *State) CreateTopicWillMessageID(topic, messageID, clientID string) error {
	return s.store.DefaultPutKey(broker.TopicWillMessageMessageIDKey(topic, messageID).String(), clientID)
}

func (s *State) ReadTopicWillMessageID(topic string) (map[string]string, error) {
	var (
		messageIDClientID = map[string]string{}
		value, err        = s.store.DefaultReadPrefixKey(context.TODO(), broker.TopicWillMessage(topic).String())
	)
	if err != nil {
		return messageIDClientID, err
	}
	for k, v := range value {
		messageIDClientID[k] = v
	}
	return messageIDClientID, err
}

func (s *State) DeleteTopicWillMessageID(topic, messageID string) error {
	return s.store.DefaultDeleteKey(broker.TopicWillMessageMessageIDKey(topic, messageID).String())
}

func (s *State) ReadRetainMessageID(topic string) ([]string, error) {
	var (
		messageIDs []string
		value, err = s.store.DefaultReadPrefixKey(context.TODO(), broker.TopicRetainMessage(topic).String())
	)
	if err != nil {
		return messageIDs, err
	}
	for _, v := range value {
		messageIDs = append(messageIDs, v)
	}
	return messageIDs, err
}

func (s *State) CreateRetainMessageID(topic, messageID string) error {
	return s.store.DefaultPutKey(broker.TopicRetainMessage(topic).String(), messageID)
}

func (s *State) DeleteRetainMessageID(topic string) error {
	return s.store.DefaultDeleteKey(broker.TopicRetainMessage(topic).String())
}
