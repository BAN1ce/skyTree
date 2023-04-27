package controller

import (
	"github.com/BAN1ce/skyTree/inner/api/response"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/labstack/echo/v4"
)

type Client struct {
	manager *client.Manager
}

func NewClient(manager *client.Manager) *Client {
	return &Client{
		manager: manager,
	}
}

func (c *Client) Get(ctx echo.Context) error {
	return ctx.JSON(200, response.WithData(c.manager.Info()))
}

func (c *Client) Delete(ctx echo.Context) error {
	if cl, ok := c.manager.ReadClient(ctx.Param("id")); ok {
		c.manager.DeleteClient(cl)
		return ctx.JSON(200, response.WithSuccess())

	}
	return ctx.JSON(200, response.WithCode(response.NotExists))
}
