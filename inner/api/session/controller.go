package session

import (
	"github.com/BAN1ce/skyTree/inner/api/base"
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/labstack/echo/v4"
)

type Manager interface {
	ReadSession(key string) (broker.Session, bool)
}
type Controller struct {
	manager Manager
}

func NewController(manager Manager) *Controller {
	return &Controller{
		manager: manager,
	}

}

func (c *Controller) Info(ctx echo.Context) error {
	var (
		req = InfoRequest{}
		err error
	)
	if err = ctx.Bind(&req); err != nil {
		return err
	}
	session, ok := c.manager.ReadSession(req.ClientID)
	if !ok {
		return ctx.JSON(200, base.WithCode(base.CodeClientNotExists))
	}
	var data infoData

	for topic, qos := range session.ReadSubTopics() {
		lastMessageID, _ := session.ReadTopicLatestPushedMessageID(topic)
		data.SubTopic = append(data.SubTopic, subTopic{
			Topic:         topic,
			Qos:           int(qos),
			LastMessageID: lastMessageID,
		})
	}
	data.ClientID = req.ClientID
	return ctx.JSON(200, withInfoData(data))
}
