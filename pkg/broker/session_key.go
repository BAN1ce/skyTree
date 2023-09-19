package broker

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

func ClientKey(clientID string) *strings.Builder {
	var build = strings.Builder{}
	build.WriteString(KeyClientPrefix)
	build.WriteString(clientID)
	return &build

}
func WithClientKey(key, clientID string) string {
	build := ClientKey(clientID)
	build.WriteString("/")
	build.WriteString(key)
	return build.String()
}
func ClientSubTopicKeyPrefix(clientID string) string {
	var (
		build = ClientKey(clientID)
	)
	build.WriteString(KeyClientSubTopic)
	build.WriteString("/")
	return build.String()
}

func ClientSubTopicKey(clientID, topic string) string {
	var (
		build = ClientKey(clientID)
	)
	build.WriteString(KeyClientSubTopic)
	build.WriteString("/")
	build.WriteString(topic)
	return build.String()
}

func ClientTopicUnAckKeyPrefix(clientID, topic string) string {
	var (
		build = ClientKey(clientID)
	)
	build.WriteString(KeyClientUnAckMessageID)
	build.WriteString("/")
	build.WriteString(topic)
	return build.String()
}

func ClientTopicUnFinishedMessagePrefix(clientID, topic string) string {
	var (
		build = ClientKey(clientID)
	)
	build.WriteString(keyClientUnfinishedMessage)
	build.WriteString("/")
	build.WriteString(topic)
	return build.String()

}

func ClientTopicUnAckKey(clientID, topic, messageID string) string {
	var (
		build = ClientKey(clientID)
	)
	build.WriteString(KeyClientUnAckMessageID)
	build.WriteString("/")
	build.WriteString(topic)
	build.WriteString("/")
	build.WriteString(messageID)
	return build.String()
}

func ClientConnectPropertiesKey(clientID string) string {
	var (
		build = ClientKey(clientID)
	)
	build.WriteString(KeyConnectProperties)
	return build.String()
}

func ClientWillMessageKey(clientID string) string {
	var (
		build = ClientKey(clientID)
	)
	build.WriteString(KeyWillMessage)
	return build.String()
}
