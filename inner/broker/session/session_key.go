package session

func clientSubTopicKeyPrefix(clientID string) string {
	var (
		build = clientKey(clientID)
	)
	build.WriteString(KeyClientSubTopic)
	build.WriteString("/")
	return build.String()
}

func clientSubTopicKey(clientID, topic string) string {
	var (
		build = clientKey(clientID)
	)
	build.WriteString(KeyClientSubTopic)
	build.WriteString("/")
	build.WriteString(topic)
	return build.String()
}

func clientTopicUnAckKeyPrefix(clientID, topic string) string {
	var (
		build = clientKey(clientID)
	)
	build.WriteString(KeyClientUnAckMessageID)
	build.WriteString("/")
	build.WriteString(topic)
	return build.String()
}

func clientTopicUnFinishedMessagePrefix(clientID, topic string) string {
	var (
		build = clientKey(clientID)
	)
	build.WriteString(keyClientUnfinishedMessage)
	build.WriteString("/")
	build.WriteString(topic)
	return build.String()

}

func clientTopicUnAckKey(clientID, topic, messageID string) string {
	var (
		build = clientKey(clientID)
	)
	build.WriteString(KeyClientUnAckMessageID)
	build.WriteString("/")
	build.WriteString(topic)
	build.WriteString("/")
	build.WriteString(messageID)
	return build.String()
}
