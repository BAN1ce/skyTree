package session

type subTopic struct {
	Topic          string   `json:"topic"`
	Qos            int      `json:"qos"`
	LastMessageID  string   `json:"last_message_id"`
	UnAckMessageID []string `json:"un_ack_message_id"`
}

type infoData struct {
	ClientID string     `json:"client_id"`
	SubTopic []subTopic `json:"sub_topic"`
}
