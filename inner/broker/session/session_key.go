package session

import "strings"

const (
	KeyClientPrefix               = "client/"
	KeyClientUnAckMessageID       = `/unack/message_id`
	keyClientUnfinishedMessage    = `/unfinished/message`
	KeyClientUnRecPacketID        = `/unrec/packet_id`
	KeyClientUnCompPacketID       = `/uncomp/packet_id`
	KeyClientLatestAliveTime      = `/latest_alive_time`
	KeyClientLatestAckedMessageID = `/latest_acked_message_id`
	KeyClientSubTopic             = `/sub_topic`
	KeyConnectProperties          = `/connect_properties`
	KeyWillMessage                = `/will_message`
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

func clientConnectPropertiesKey(clientID string) string {
	var (
		build = clientKey(clientID)
	)
	build.WriteString(KeyConnectProperties)
	return build.String()
}

func clientWillMessageKey(clientID string) string {
	var (
		build = clientKey(clientID)
	)
	build.WriteString(KeyWillMessage)
	return build.String()
}
