package client

import (
	"github.com/BAN1ce/skyTree/api/base"
	"github.com/BAN1ce/skyTree/inner/broker/core"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Controller struct {
	core.ClientManager
}

func NewController(manager *core.ClientManager) *Controller {
	return &Controller{
		ClientManager: *manager,
	}
}

func (c *Controller) Info(ctx echo.Context) error {
	//var (
	//	req InfoRequest
	//	err error
	//)
	//if err = ctx.Bind(&req); err != nil {
	//	return err
	//}
	//client, ok := c.ReadClient(req.ID)
	//if !ok {
	//	return ctx.JSON(http.StatusOK, base.WithCode(api.CodeNotFound))
	//} else {
	//	//return ctx.JSON(http.StatusOK, base.WithData(client))
	//}
	return nil
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
