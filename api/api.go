package api

import (
	"context"
	client2 "github.com/BAN1ce/skyTree/api/client"
	"github.com/BAN1ce/skyTree/api/topic"
	_ "github.com/BAN1ce/skyTree/docs"
	broker2 "github.com/BAN1ce/skyTree/pkg/broker"
	session2 "github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	_ "net/http/pprof"
)

type Option func(*API)

func WithClientManager(manager client2.Manager) Option {
	return func(api *API) {
		api.manager = manager
	}
}

func WithTopicManager(topicManager topic.Manager) Option {
	return func(api *API) {
		api.topicManager = topicManager
	}
}

func WithSessionManager(sessionManager session2.Manager) Option {
	return func(api *API) {
		api.sessionManager = sessionManager
	}
}

func WithStore(store broker2.MessageStore) Option {
	return func(api *API) {
		api.store = store
	}
}

type API struct {
	addr           string
	httpServer     *gin.Engine
	manager        client2.Manager
	topicManager   topic.Manager
	store          broker2.MessageStore
	sessionManager session2.Manager
	apiV1          *gin.RouterGroup
}

func NewAPI(addr string, option ...Option) *API {
	api := &API{
		addr: addr,
	}
	for _, opt := range option {
		opt(api)
	}
	return api
}
func (a *API) Start(ctx context.Context) error {
	r := gin.Default()
	a.httpServer = r
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "ok",
		})
	})
	a.route()
	return r.Run(a.addr)
}

func (a *API) route() {
	a.httpServer.GET("/debug/pprof/*filepath", gin.WrapH(http.DefaultServeMux))
	a.httpServer.GET("/metrics", gin.WrapH(promhttp.Handler()))
	a.apiV1 = a.httpServer.Group("/api/v1")
	a.clientAPI()
}

func (a *API) clientAPI() {
	ctr := client2.NewController(a.manager)
	a.apiV1.GET("/clients/:client_id", ctr.Info)
}

func (a *API) topicAPI() {
	ctr := topic.NewController(a.topicManager)
	a.apiV1.GET("/topics/:topic", ctr.Info)
}
func (a *API) Name() string {
	return "api"
}
