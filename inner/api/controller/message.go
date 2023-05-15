package controller

import (
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/labstack/echo/v4"
)

type Message struct {
	store pkg.Store
}

func NewMessage(store pkg.Store) *Message {
	return &Message{
		store: store,
	}
}

func (m *Message) Info(ctx echo.Context) error {

	return nil
}
