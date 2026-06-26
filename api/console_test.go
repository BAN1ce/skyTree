package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/gin-gonic/gin"
)

func TestConsoleRoutesDisabledWhenConfigDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := NewAPI(":0", &Component{
		Console: fakeConsoleProvider{},
	})
	a.httpServer = gin.New()
	a.route()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/console/summary", nil)
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected disabled console API route to return 404, got %d", rec.Code)
	}
}

func TestConsoleRoutesRequireBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := newConsoleTestAPI()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/console/summary", nil)
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected console API to require auth, got %d", rec.Code)
	}

	webReq := httptest.NewRequest(http.MethodGet, "/console", nil)
	webRec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(webRec, webReq)
	if webRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected console web to require auth, got %d", webRec.Code)
	}
}

func TestConsoleSummaryReturnsDataWithBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := newConsoleTestAPI()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/console/summary", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected summary to return 200, got %d", rec.Code)
	}
}

func TestConsoleWebReturnsIndexWithBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := newConsoleTestAPI()
	req := httptest.NewRequest(http.MethodGet, "/console", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected console web to return 200, got %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("expected console web response body")
	}
}

func TestConsoleClusterNodesReturnsDataWithBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := newConsoleTestAPI()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/console/cluster/nodes", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected cluster nodes to return 200, got %d", rec.Code)
	}
}

func TestConsoleShareGroupMembersReturnsDataWithBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := newConsoleTestAPI()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/console/share-groups/group-a/members", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected share group members to return 200, got %d", rec.Code)
	}
}

func TestConsoleClusterNodeActionRequiresBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := newConsoleTestAPI()
	req := newClusterActionTestRequest("node-1", ConsoleClusterNodeAction{Action: "restart"})
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected cluster action to require auth, got %d", rec.Code)
	}
}

func TestConsoleClusterNodeActionDispatchesWithBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := newConsoleTestAPI()
	req := newClusterActionTestRequest("node-1", ConsoleClusterNodeAction{Action: "restart"})
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected cluster action to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestConsoleClusterNodeJoinActionDispatchesWithBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.LoadForTest()

	a := newConsoleTestAPI()
	req := newClusterActionTestRequest("skytree-3", ConsoleClusterNodeAction{Action: "join"})
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
	rec := httptest.NewRecorder()
	a.httpServer.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected cluster join action to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func newConsoleTestAPI() *API {
	a := NewAPI(":0", &Component{
		Console:         fakeConsoleProvider{},
		ConsoleEnabled:  true,
		ConsoleUsername: "admin",
		ConsolePassword: "secret",
	})
	a.httpServer = gin.New()
	a.route()
	return a
}

type fakeConsoleProvider struct{}

func (fakeConsoleProvider) GetSummary(context.Context) (*ConsoleSummary, error) {
	return &ConsoleSummary{
		Timestamp:     time.Unix(1, 0),
		ServerPort:    9526,
		MetricsPath:   "/metrics",
		OnlineClients: 1,
	}, nil
}

func (fakeConsoleProvider) ListClients(context.Context, ConsoleClientQuery) (*ConsoleClientList, error) {
	return &ConsoleClientList{Items: []ConsoleClientSummary{}}, nil
}

func (fakeConsoleProvider) GetClient(context.Context, string) (*ConsoleClientDetail, bool, error) {
	return nil, false, nil
}

func (fakeConsoleProvider) GetSubscriptionTree(
	context.Context,
	ConsoleSubscriptionTreeQuery,
) (*ConsoleSubscriptionTree, error) {
	return &ConsoleSubscriptionTree{}, nil
}

func (fakeConsoleProvider) GetShareGroupMembers(context.Context, string) (*ConsoleShareGroupMemberList, error) {
	return &ConsoleShareGroupMemberList{
		Total: 1,
		Items: []ConsoleShareGroupMember{
			{ClientID: "client-a", TopicFilter: "topic/a"},
		},
	}, nil
}

func (fakeConsoleProvider) ListRetainMessages(context.Context, ConsoleRetainQuery) (*ConsoleRetainList, error) {
	return &ConsoleRetainList{Items: []ConsoleRetainItem{}}, nil
}

func (fakeConsoleProvider) ListDueWillDelayTasks(context.Context, ConsoleWillDelayQuery) (*ConsoleWillDelayList, error) {
	return &ConsoleWillDelayList{Items: []ConsoleWillDelayTask{}}, nil
}

func (fakeConsoleProvider) ListClusterNodes(context.Context) (*ConsoleClusterNodeList, error) {
	return &ConsoleClusterNodeList{
		RegisteredNodes: []ConsoleRegisteredNode{
			{NodeID: 1, GRPCEndpoint: "skytree-node1:53001"},
		},
	}, nil
}

func (fakeConsoleProvider) RunClusterNodeAction(
	context.Context,
	string,
	ConsoleClusterNodeAction,
) (*ConsoleClusterNodeActionResult, error) {
	return &ConsoleClusterNodeActionResult{
		Node:   "node-1",
		Action: "restart",
		Status: "accepted",
	}, nil
}

func newClusterActionTestRequest(node string, action ConsoleClusterNodeAction) *http.Request {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(action); err != nil {
		panic(err)
	}
	return httptest.NewRequest(http.MethodPost, "/api/v1/console/cluster/nodes/"+node+"/actions", &body)
}
