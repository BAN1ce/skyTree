package session

const (
	keySessionPrefix = "session/client/"
	keyTopicPrefix   = "/topic"

	keyClientLatestAliveTime = `/latest_alive_time`
	keySubTopic              = `/sub_topic`
	keyConnectProperties     = `/connect_properties`
	keyWillMessage           = `/will_message`

	keyClientUnfinishedMessage    = `/unfinished/message`
	keyClientLatestAckedMessageID = `/latest_message_id`
	KeyCreatedTime                = `/created_time`
)

func clientSessionPrefix(clientID string) string {
	return keySessionPrefix + clientID
}

func clientSessionCreatedTimeKey(clientID string) string {
	return clientSessionPrefix(clientID) + KeyCreatedTime
}
func clientTopicPrefix(clientID string) string {
	return clientSessionPrefix(clientID) + keyTopicPrefix
}

func clientUnfinishedMessageKey(clientID, topic string) string {
	return clientTopicPrefix(clientID) + keyClientUnfinishedMessage + "/" + topic

}
func clientLatestAckedMessageKey(clientID, topic string) string {
	return clientTopicPrefix(clientID) + keyClientLatestAckedMessageID + "/" + topic
}

func clientSubTopicKeyPrefix(clientID string) string {
	return clientTopicPrefix(clientID) + keySubTopic

}
func clientSubTopicKey(clientID, topic string) string {
	return clientSubTopicKeyPrefix(clientID) + "/" + topic
}

func clientWillMessageKey(clientID string) string {
	return clientSessionPrefix(clientID) + keyWillMessage
}

func clientConnectPropertiesKey(clientID string) string {
	return clientSessionPrefix(clientID) + keyConnectProperties
}
