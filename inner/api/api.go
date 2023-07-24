package api

import (
	"context"
	_ "github.com/BAN1ce/skyTree/docs"
	"github.com/BAN1ce/skyTree/inner/api/base"
	"github.com/BAN1ce/skyTree/inner/api/session"
	"github.com/BAN1ce/skyTree/inner/api/store"
	"github.com/BAN1ce/skyTree/inner/broker"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"
	"go.uber.org/zap"
)

type Option func(*API)

func WithClientManager(manager *broker.Manager) Option {
	return func(api *API) {
		api.manager = manager
	}
}

func WithSessionManager(sessionManager pkg.SessionManager) Option {
	return func(api *API) {
		api.sessionManager = sessionManager
	}
}

func WithStore(store pkg.Store) Option {
	return func(api *API) {
		api.store = store
	}
}

type API struct {
	addr           string
	httpServer     *echo.Echo
	apiV1          *echo.Group
	manager        *broker.Manager
	store          pkg.Store
	sessionManager pkg.SessionManager
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
	a.httpServer.HTTPErrorHandler = func(err error, ctx echo.Context) {
		if err := ctx.JSON(500, base.WithError(err)); err != nil {
			logger.Logger.Error("http error handler error", zap.Error(err))
		}
	}
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
	a.storeAPI()
	a.sessionAPI()
}

func (a *API) storeAPI() {
	ctr := store.NewController(a.store)
	a.apiV1.GET("/store", ctr.Get)
}
func (a *API) sessionAPI() {
	ctr := session.NewController(a.sessionManager)
	a.apiV1.GET("/client.proto/:client_id", ctr.Info)

}
func (a *API) Name() string {
	return "api"
}
