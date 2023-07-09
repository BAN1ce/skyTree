package store

import (
	"context"
	"github.com/BAN1ce/skyTree/inner/api/base"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/labstack/echo/v4"
	"time"
)

type Controller struct {
	pkg.ClientMessageStore
}

func NewController(store pkg.ClientMessageStore) *Controller {
	return &Controller{
		ClientMessageStore: store,
	}
}

func (c *Controller) Get(ctx echo.Context) error {
	var (
		req       getRequest
		messageID []string
		err       error
	)
	if err = ctx.Bind(&req); err != nil {
		return err
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	message, err := c.ReadFromTimestamp(context.TODO(), req.Topic, time.Time{}, req.PageSize)
	if err != nil {
		return err
	}
	for _, v := range message {
		messageID = append(messageID, v.MessageID)
	}
	return ctx.JSON(200, base.WithData(getData{
		Page: base.Page{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    len(messageID),
		},
		Data: messageID,
	}))
}
