package store

import (
	"context"
	"errors"
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

func (c *Controller) Info(ctx echo.Context) error {
	var (
		messageID        = ctx.Param("id")
		topic            = ctx.QueryParam("topic")
		ctxInner, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	)
	defer cancel()
	if message, err := c.TopicMessageStore.ReadTopicMessagesByID(ctxInner, topic, messageID, 1, true); err != nil {
		return err
	} else {
		if len(message) == 0 {
			return errors.New("no message")
		}
		msg := message[0]
		return ctx.JSON(200, newInfoResponse(InfoData{
			MessageID:   messageID,
			Payload:     msg.PublishPacket.Payload,
			PubReceived: msg.PubReceived,
			FromSession: msg.IsFromSession(),
			TimeStamp:   msg.Timestamp,
			ExpiredTime: msg.ExpiredTime,
			Will:        msg.Will,
		}))
	}
}
