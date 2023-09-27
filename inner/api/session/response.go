package session

import "github.com/BAN1ce/skyTree/inner/api/base"

type subTopic struct {
	Topic          string   `json:"topic"`
	Qos            int      `json:"qos"`
	LastMessageID  string   `json:"last_message_id,omitempty"`
	UnAckMessageID []string `json:"un_ack_message_id,omitempty"`
}

type infoData struct {
	ClientID string     `json:"client_id"`
	SubTopic []subTopic `json:"sub_topic"`
}

type InfoResponse struct {
	*base.Response
	Data infoData `json:"data"`
}

func withInfoData(data infoData) *InfoResponse {
	return &InfoResponse{
		Response: base.WithSuccess(),
		Data:     data,
	}

}
