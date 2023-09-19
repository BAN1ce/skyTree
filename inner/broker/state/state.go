package state

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
	"time"
)

type State struct {
	store *broker.KeyValueStoreWithTimeout
}

func NewState(store broker.KeyValueStore) *State {
	return &State{
		store: broker.NewKeyValueStoreWithTimout(store, 3*time.Second),
	}
}

func (s *State) CreateTopicWillMessageID(topic, messageID string, retain bool) error {
	if retain {
		return s.store.DefaultPutKey(broker.TopicWillMessageMessageIDKey(topic, messageID), "true")
	} else {
		return s.store.DefaultPutKey(broker.TopicWillMessageMessageIDKey(topic, messageID), "false")
	}
}

func (s *State) ReadTopicWillMessageID(topic string) ([]string, error) {
	var (
		id         []string
		value, err = s.store.DefaultReadPrefixKey(broker.TopicWillMessage(topic))
	)
	if err != nil {
		return id, err
	}
	for k := range value {
		id = append(id, k)
	}
	return id, err
}

func (s *State) DeleteTopicWillMessageID(topic, messageID string) error {
	return s.store.DefaultDeleteKey(broker.TopicWillMessageMessageIDKey(topic, messageID))
}
