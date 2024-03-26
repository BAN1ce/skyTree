package topic

import (
	"github.com/BAN1ce/skyTree/api/base"
	"github.com/BAN1ce/skyTree/pkg/model"
	"github.com/gin-gonic/gin"
)

type Manager interface {
	ReadTopic(topic string) (*model.Topic, error)
}
type Controller struct {
	topics Manager
}

func NewController(topics Manager) *Controller {
	return &Controller{
		topics: topics,
	}
}

func (c *Controller) Info(g *gin.Context) {
	var (
		req     InfoRequest
		rspData InfoResponseData
	)
	if err := g.ShouldBindUri(&req); err != nil {
		g.Error(err)
		return
	}
	topic, err := c.topics.ReadTopic(req.Topic)
	if err != nil {
		g.Error(err)
		return
	}
	rspData.Topic = *topic
	g.JSON(200, base.WithData(rspData))
}
