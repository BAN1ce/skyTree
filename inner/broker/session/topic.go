package session

// topicMeta is the metadata of a topic
type topicMeta struct {
	topic            string
	qos              int32
	unAckMessageID   map[string]struct{}
	lastAckMessageID string
}

func newTopicMeta(topic string, qos int32) *topicMeta {
	return &topicMeta{
		topic: topic,
		qos:   qos,
	}
}

type subTopicsMeta struct {
	topics map[string]*topicMeta
}

func newSubTopicsMeta() *subTopicsMeta {
	return &subTopicsMeta{
		topics: make(map[string]*topicMeta),
	}
}

func (s *subTopicsMeta) ReadSubTopics() map[string]int32 {
	m := make(map[string]int32)
	for k, v := range s.topics {
		m[k] = v.qos
	}
	return m
}

func (s *subTopicsMeta) CreateSubTopics(topic string, qos int32) {
	s.topics[topic] = newTopicMeta(topic, qos)
}

func (s *subTopicsMeta) DeleteSubTopics(topic string) {
	delete(s.topics, topic)
}

func (s *subTopicsMeta) UpdateTopicLastAckedMessageID(topic string, messageID string) {
	if topicMeta, ok := s.topics[topic]; !ok {
		return
	} else {
		topicMeta.lastAckMessageID = messageID
	}
}

func (s *subTopicsMeta) ReadTopicLastAckedMessageID(topic string) (string, bool) {
	if topicMeta, ok := s.topics[topic]; !ok {
		return "", false
	} else {
		if topicMeta.lastAckMessageID == "" {
			return "", false
		}
		return topicMeta.lastAckMessageID, true
	}
}

func (s *subTopicsMeta) CreateTopicUnAckMessageID(topic string, messageID []string) {
	if topicMeta, ok := s.topics[topic]; !ok {
		return
	} else {
		if topicMeta.unAckMessageID == nil {
			topicMeta.unAckMessageID = make(map[string]struct{})

		}
		for _, id := range messageID {
			topicMeta.unAckMessageID[id] = struct{}{}
		}
	}
}

func (s *subTopicsMeta) DeleteTopicUnAckMessageID(topic string, messageID string) {
	if topicMeta, ok := s.topics[topic]; !ok {
		return
	} else {
		for id := range topicMeta.unAckMessageID {
			if id == messageID {
				delete(topicMeta.unAckMessageID, id)
				break
			}
		}
	}
}

func (s *subTopicsMeta) ReadSubTopicsLastAckedMessageID() map[string]string {
	m := make(map[string]string)
	for k, v := range s.topics {
		m[k] = v.lastAckMessageID
	}
	return m
}
