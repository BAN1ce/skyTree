package controller

import (
	"github.com/BAN1ce/skyTree/inner/broker"
	"github.com/labstack/echo/v4"
)

type Client struct {
}

func NewClient(manager *broker.Manager) *Client {
	return &Client{}
}

func (c *Client) Get(ctx echo.Context) error {
	return nil
}

func (c *Client) Delete(ctx echo.Context) error {
	return nil
}
