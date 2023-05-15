package api

import (
	"context"
	_ "github.com/BAN1ce/skyTree/docs"
	"github.com/BAN1ce/skyTree/inner/api/controller"
	"github.com/BAN1ce/skyTree/inner/broker/client"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

type Option func(*API)

func WithClientManager(manager *client.Manager) Option {
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
	manager    *client.Manager
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
	a.apiV1 = a.httpServer.Group("/api/v1")
	a.apiV1.GET("/ping", func(ctx echo.Context) error {
		return ctx.String(200, "pong")
	})
	a.client()
	a.message()
}

func (a *API) client() {
	var (
		ctr = controller.NewClient(a.manager)
	)
	a.apiV1.GET("/clients", ctr.Get)
	a.apiV1.DELETE("/clients/:id", ctr.Delete)
}

func (a *API) message() {
	var (
		ctr = controller.NewMessage(a.store)
	)
	a.apiV1.GET("/message", ctr.Info)
}
func (a *API) Name() string {
	return "api"
}
