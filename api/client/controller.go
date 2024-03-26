package client

import (
	"fmt"
	"github.com/BAN1ce/skyTree/api/base"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/BAN1ce/skyTree/pkg/model"
	"github.com/gin-gonic/gin"
)

type Manager interface {
	ReadClient(clientID string) (*client.Client, bool)
}
type Controller struct {
	clients Manager
}

func NewController(clients Manager) *Controller {
	return &Controller{
		clients: clients,
	}
}

func (c *Controller) Info(g *gin.Context) {
	var (
		req     InfoReq
		rspData model.Client
	)
	if err := g.ShouldBindUri(&req); err != nil {
		g.Error(err)
		return
	}
	client, ok := c.clients.ReadClient(req.ClientID)
	if !ok {
		// TODO: use error value
		g.Error(fmt.Errorf("client not found"))
		return
	}
	rspData.Session = c.sessionToRsp(client.GetSession())
	rspData.ID = client.ID
	rspData.UID = client.UID
	rspData.Meta = client.Meta()
	g.JSON(200, base.WithData(rspData))
}

func (c *Controller) sessionToRsp(s session.Session) *model.Session {
	var (
		result = &model.Session{}
	)
	result.ExpiryInterval = s.GetExpiryInterval()
	if properties, err := s.GetConnectProperties(); err == nil {
		result.ConnectProperty = properties
	}
	if will, ok, err := s.GetWillMessage(); ok && err == nil {
		result.WillMessage = will
	}
	topics := s.ReadSubTopics()
	result.Topic = topics

	return result
}

func (c *Controller) clientInfoToRsp() {

}
