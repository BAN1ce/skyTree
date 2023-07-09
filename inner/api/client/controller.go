package client

import (
	"github.com/BAN1ce/skyTree/inner/api"
	"github.com/BAN1ce/skyTree/inner/api/base"
	"github.com/BAN1ce/skyTree/inner/broker"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Controller struct {
	broker.Manager
}

func NewController(manager *broker.Manager) *Controller {
	return &Controller{
		Manager: *manager,
	}
}

func (c *Controller) Info(ctx echo.Context) error {
	var (
		req InfoRequest
		err error
	)
	if err = ctx.Bind(&req); err != nil {
		return err
	}
	client, ok := c.ReadClient(req.ID)
	if !ok {
		return ctx.JSON(http.StatusOK, base.WithCode(api.CodeNotFound))
	} else {
		return ctx.JSON(http.StatusOK, base.WithData(client))
	}
}

func (c *Controller) Delete(ctx echo.Context) error {
	var (
		req DeleteRequest
		err error
	)
	if err = ctx.Bind(&req); err != nil {
		return err
	}
	c.DeleteClient(req.ID)
	return ctx.JSON(http.StatusOK, base.WithSuccess())
}
