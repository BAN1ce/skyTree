package client

import (
	"github.com/BAN1ce/skyTree/pkg/model"
)

type InfoResponseData struct {
	Session *model.Session `json:"session,omitempty"`
	ID      string         `json:"ID,omitempty"`
	UID     string         `json:"UID,omitempty"`
}
