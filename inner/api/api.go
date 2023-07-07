package api

import (
	"context"
	_ "github.com/BAN1ce/skyTree/docs"
	"github.com/BAN1ce/skyTree/inner/broker"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"
)

type Option func(*API)

func WithClientManager(manager *broker.Manager) Option {
	return func(api *API) {
		api.manager = manager
	}
}
func WithStore(store pkg.Store) Option {
	return func(api *API) {
		api.store = store
	}
}

type API struct {
	addr       string
	httpServer *echo.Echo
	apiV1      *echo.Group
	manager    *broker.Manager
	store      pkg.Store
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
	a.httpServer = echo.New()
	a.httpServer.Debug = false
	a.httpServer.HideBanner = true
	a.route()
	return a.httpServer.Start(a.addr)
}

func (a *API) route() {
	a.httpServer.GET("/swagger/*", echoSwagger.WrapHandler)
	a.httpServer.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	a.apiV1 = a.httpServer.Group("/api/v1")
	a.apiV1.GET("/ping", func(ctx echo.Context) error {
		return ctx.String(200, "pong")
	})
	a.client()
	a.message()
}

func (a *API) client() {
	// var (
	// 	ctr = client.NewController(a.manager)
	// )
	// a.apiV1.GET("/clients/:id", ctr.Info)
	// a.apiV1.DELETE("/clients/:id", ctr.Delete)
}

func (a *API) message() {

}
func (a *API) Name() string {
	return "api"
}
