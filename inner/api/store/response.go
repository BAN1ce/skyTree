package store

import (
	"github.com/BAN1ce/skyTree/inner/api/base"
)

type getData struct {
	Page base.Page   `json:"page"`
	Data interface{} `json:"data"`
}
