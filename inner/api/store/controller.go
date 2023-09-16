package store

import (
	"context"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/labstack/echo/v4"
	"time"
)

type Controller struct {
	broker.TopicMessageStore
}

func NewController(store broker.TopicMessageStore) *Controller {
	return &Controller{
		TopicMessageStore: store,
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
	return nil
	//return ctx.JSON(200, base.WithData(getData{
	//	Page: base.Page{
	//		Page:     req.Page,
	//		PageSize: req.PageSize,
	//		Total:    len(messageID),
	//	},
	//	Data: messageID,
	//}))
}
