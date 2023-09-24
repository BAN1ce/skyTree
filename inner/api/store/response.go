package store

import (
	"github.com/BAN1ce/skyTree/inner/api/base"
)

type getData struct {
	Page base.Page   `json:"page"`
	Data interface{} `json:"data"`
}

type InfoData struct {
	MessageID   string
	Payload     []byte
	PubReceived bool
	FromSession bool
	TimeStamp   int64
	ExpiredTime int64
	Will        bool
}
type InfoResponse struct {
	base.Response
	Data InfoData `json:"data"`
}

func newInfoResponse(data InfoData) *InfoResponse {
	return &InfoResponse{
		Response: *base.WithSuccess(),
		Data:     data,
	}

}
