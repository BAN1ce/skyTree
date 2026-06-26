package consoleruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/api"
	"github.com/BAN1ce/skyTree/config"
)

type GardenerControlClient struct {
	baseURL string
	token   string
	timeout time.Duration
	client  *http.Client
}

func NewGardenerControlClient(cfg config.ConsoleControl) *GardenerControlClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &GardenerControlClient{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		token:   cfg.Token,
		timeout: timeout,
		client:  http.DefaultClient,
	}
}

func (c *GardenerControlClient) ListClusterNodes(ctx context.Context) ([]api.ConsoleRuntimeNode, error) {
	var response gardenerClusterNodesResponse
	if err := c.do(ctx, http.MethodGet, "/api/cluster/nodes", nil, &response); err != nil {
		return nil, err
	}
	return response.Nodes, nil
}

func (c *GardenerControlClient) ListCandidateNodes(context.Context) ([]api.ConsoleCandidateNode, error) {
	return []api.ConsoleCandidateNode{}, nil
}

func (c *GardenerControlClient) RunClusterNodeAction(
	ctx context.Context,
	node string,
	action string,
) (*api.ConsoleClusterNodeActionResult, error) {
	req := gardenerClusterActionRequest{Action: action}
	var response api.ConsoleClusterNodeActionResult
	path := "/api/cluster/nodes/" + urlPathEscape(node) + "/actions"
	if err := c.do(ctx, http.MethodPost, path, req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *GardenerControlClient) do(ctx context.Context, method string, path string, in any, out any) error {
	if c == nil || c.baseURL == "" {
		return fmt.Errorf("gardener control base url is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var body *bytes.Reader
	if in == nil {
		body = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal gardener control request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create gardener control request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.token) != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpClient := c.client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call gardener control api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gardener control api returned status %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode gardener control response: %w", err)
	}
	return nil
}

type gardenerClusterNodesResponse struct {
	Nodes []api.ConsoleRuntimeNode `json:"nodes"`
}

type gardenerClusterActionRequest struct {
	Action string `json:"action"`
}

func urlPathEscape(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"/", "%2F",
		" ", "%20",
		"?", "%3F",
		"#", "%23",
	)
	return replacer.Replace(value)
}
