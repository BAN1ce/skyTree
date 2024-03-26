package sdk

import "github.com/BAN1ce/skyTree/pkg/model"

type BaseResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

type SessionResponse struct {
	BaseResponse
	Data model.Session `json:"data"`
}

type ClientResponse struct {
	BaseResponse
	Data model.Client `json:"data"`
}

type TopicResponse struct {
	BaseResponse
	Data model.Topic `json:"data"`
}
