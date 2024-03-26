package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/BAN1ce/skyTree/pkg/model"
	"github.com/go-resty/resty/v2"
)

type App struct {
	BaseURL    string
	httpClient *resty.Client
}

type Option func(app *App)

func WithHTTPClient(client *resty.Client) Option {
	return func(app *App) {
		app.httpClient = client
	}
}
func NewApp(baseURL string, op ...Option) *App {
	a := &App{
		BaseURL:    baseURL,
		httpClient: resty.New().SetBaseURL(baseURL),
	}
	for _, o := range op {
		o(a)
	}
	return a
}

func (a *App) GetClientByID(ctx context.Context, id string) (c *model.Client, err error) {
	var (
		rsp    *resty.Response
		result ClientResponse
	)
	rsp, err = a.httpClient.R().SetContext(ctx).Get("/api/v1/clients/" + id)
	if err = json.Unmarshal(rsp.Body(), &result); err != nil {
		return
	}
	if !result.Success {
		return nil, fmt.Errorf("get client failed: %d - %s", result.Code, result.Msg)
	}
	c = &result.Data
	return
}

func (a *App) GetTopic(ctx context.Context, topic string) (t *model.Topic, err error) {
	var (
		rsp    *resty.Response
		result TopicResponse
	)
	rsp, err = a.httpClient.R().SetContext(ctx).Get("/api/v1/topics/" + topic)
	if err = json.Unmarshal(rsp.Body(), &result); err != nil {
		return
	}
	if !result.Success {
		return nil, fmt.Errorf("get topic failed: %d - %s", result.Code, result.Msg)
	}
	t = &result.Data
	return

}
