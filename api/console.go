package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/api/base"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/gin-gonic/gin"
)

const defaultConsoleLimit = 100

type ConsoleProvider interface {
	GetSummary(ctx context.Context) (*ConsoleSummary, error)
	ListClients(ctx context.Context, query ConsoleClientQuery) (*ConsoleClientList, error)
	GetClient(ctx context.Context, clientID string) (*ConsoleClientDetail, bool, error)
	GetSubscriptionTree(ctx context.Context, query ConsoleSubscriptionTreeQuery) (*ConsoleSubscriptionTree, error)
	GetShareGroupMembers(ctx context.Context, shareGroup string) (*ConsoleShareGroupMemberList, error)
	ListRetainMessages(ctx context.Context, query ConsoleRetainQuery) (*ConsoleRetainList, error)
	ListDueWillDelayTasks(ctx context.Context, query ConsoleWillDelayQuery) (*ConsoleWillDelayList, error)
	ListClusterNodes(ctx context.Context) (*ConsoleClusterNodeList, error)
	RunClusterNodeAction(ctx context.Context, node string, action ConsoleClusterNodeAction) (*ConsoleClusterNodeActionResult, error)
}

type ConsoleSummary struct {
	Timestamp       time.Time                      `json:"timestamp"`
	ServerPort      int                            `json:"server_port"`
	ClusterEnabled  bool                           `json:"cluster_enabled"`
	OnlineClients   int                            `json:"online_clients"`
	StorageDriver   string                         `json:"storage_driver"`
	MetricsPath     string                         `json:"metrics_path"`
	DeliveryBacklog ClusterDeliveryBacklogOverview `json:"delivery_backlog"`
}

type ConsoleClientQuery struct {
	ClientID string
	Limit    int
}

type ConsoleClientList struct {
	Total int                    `json:"total"`
	Items []ConsoleClientSummary `json:"items"`
}

type ConsoleClientSummary struct {
	ClientID        string `json:"client_id"`
	Username        string `json:"username,omitempty"`
	RemoteAddr      string `json:"remote_addr,omitempty"`
	KeepAliveSecond int64  `json:"keep_alive_second"`
	IdleSecond      int64  `json:"idle_second"`
	Online          bool   `json:"online"`
}

type ConsoleClientDetail struct {
	Client        ConsoleClientSummary         `json:"client"`
	Session       *ConsoleSessionSummary       `json:"session,omitempty"`
	Owner         *ConsoleSessionOwnerSummary  `json:"owner,omitempty"`
	Subscriptions []ConsoleSubscriptionSummary `json:"subscriptions"`
	Delivery      *ConsoleDeliverySummary      `json:"delivery,omitempty"`
}

type ConsoleSessionSummary struct {
	ClientID                string                         `json:"client_id"`
	Exists                  bool                           `json:"exists"`
	SessionExpirySecond     uint32                         `json:"session_expiry_second"`
	ExpireAtUnixNano        int64                          `json:"expire_at_unix_nano,omitempty"`
	WillMessage             *ConsoleWillMessageSummary     `json:"will_message,omitempty"`
	UnfinishedMessageCount  int                            `json:"unfinished_message_count"`
	UnfinishedMessageSample []ConsoleUnfinishedMessageItem `json:"unfinished_message_sample"`
}

type ConsoleWillMessageSummary struct {
	Topic             string `json:"topic"`
	QoS               uint32 `json:"qos"`
	Retain            bool   `json:"retain"`
	PayloadBytes      int    `json:"payload_bytes"`
	WillDelaySecond   uint32 `json:"will_delay_second,omitempty"`
	MessageExpirySecs uint32 `json:"message_expiry_second,omitempty"`
}

type ConsoleUnfinishedMessageItem struct {
	MessageID      string `json:"message_id"`
	PacketID       uint32 `json:"packet_id"`
	QoS            uint32 `json:"qos"`
	State          string `json:"state"`
	IsOutgoing     bool   `json:"is_outgoing"`
	SubscribeTopic string `json:"subscribe_topic,omitempty"`
}

type ConsoleSessionOwnerSummary struct {
	ClientID string `json:"client_id"`
	NodeID   uint64 `json:"node_id"`
	Online   bool   `json:"online"`
}

type ConsoleSubscriptionSummary struct {
	ClientID               string `json:"client_id,omitempty"`
	Topic                  string `json:"topic"`
	QoS                    int32  `json:"qos"`
	NoLocal                bool   `json:"no_local"`
	RetainAsPublished      bool   `json:"retain_as_published"`
	RetainHandling         int32  `json:"retain_handling"`
	SubscriptionIdentifier int32  `json:"subscription_identifier,omitempty"`
}

type ConsoleShareGroupMemberList struct {
	Total int                       `json:"total"`
	Items []ConsoleShareGroupMember `json:"items"`
}

type ConsoleShareGroupMember struct {
	ClientID    string `json:"client_id"`
	TopicFilter string `json:"topic_filter"`
}

type ConsoleDeliverySummary struct {
	Cursor *ConsoleDeliveryCursor `json:"cursor,omitempty"`
	Tasks  []ConsoleDeliveryTask  `json:"tasks"`
}

type ConsoleDeliveryCursor struct {
	ClientID   string    `json:"client_id"`
	Generation int64     `json:"generation"`
	LastTS     time.Time `json:"last_ts"`
	LastTaskID string    `json:"last_task_id"`
	UpdatedTS  time.Time `json:"updated_ts"`
}

type ConsoleDeliveryTask struct {
	TaskID          string    `json:"task_id"`
	MessageID       string    `json:"message_id"`
	CreatedAt       time.Time `json:"created_at"`
	DeliveryQoS     int       `json:"delivery_qos"`
	Generation      int64     `json:"generation"`
	ShareGroup      string    `json:"share_group,omitempty"`
	SubscriptionIDs []int32   `json:"subscription_ids"`
}

type ConsoleSubscriptionTreeQuery struct {
	Topic    string
	MaxDepth int32
}

type ConsoleSubscriptionTree struct {
	Root *ConsoleSubscriptionTreeNode `json:"root,omitempty"`
}

type ConsoleSubscriptionTreeNode struct {
	TopicSection string                        `json:"topic_section"`
	Topic        string                        `json:"topic"`
	Clients      []ConsoleSubscriptionSummary  `json:"clients"`
	Children     []ConsoleSubscriptionTreeNode `json:"children"`
}

type ConsoleRetainQuery struct {
	TopicFilter string
	Limit       int
}

type ConsoleRetainList struct {
	Total int                 `json:"total"`
	Items []ConsoleRetainItem `json:"items"`
}

type ConsoleRetainItem struct {
	Topic             string `json:"topic"`
	QoS               int32  `json:"qos"`
	PayloadBytes      int    `json:"payload_bytes"`
	PayloadPreview    string `json:"payload_preview,omitempty"`
	CreatedAtUnixNano int64  `json:"created_at_unix_nano,omitempty"`
	ExpiredAtUnixNano int64  `json:"expired_at_unix_nano,omitempty"`
	PublisherClientID string `json:"publisher_client_id,omitempty"`
}

type ConsoleWillDelayQuery struct {
	BeforeUnixMicro int64
	Limit           int
}

type ConsoleWillDelayList struct {
	Total int                    `json:"total"`
	Items []ConsoleWillDelayTask `json:"items"`
}

type ConsoleWillDelayTask struct {
	ClientID              string `json:"client_id"`
	ScheduledAtUnixMicro  int64  `json:"scheduled_at_unix_micro"`
	ScheduledDelaySeconds int64  `json:"scheduled_delay_seconds"`
}

type ConsoleClusterNodeList struct {
	RegisteredNodes  []ConsoleRegisteredNode `json:"registered_nodes"`
	RuntimeNodes     []ConsoleRuntimeNode    `json:"runtime_nodes"`
	CandidateNodes   []ConsoleCandidateNode  `json:"candidate_nodes"`
	ControlAvailable bool                    `json:"control_available"`
	ControlError     string                  `json:"control_error,omitempty"`
}

type ConsoleRegisteredNode struct {
	NodeID           uint64 `json:"node_id"`
	LocalNodeAddress string `json:"local_node_address"`
	GRPCAddr         string `json:"grpc_addr"`
	GRPCEndpoint     string `json:"grpc_endpoint"`
	Join             bool   `json:"join"`
}

type ConsoleRuntimeNode struct {
	Name        string `json:"name"`
	ServiceName string `json:"service_name"`
	BrokerURL   string `json:"broker_url"`
	HealthURL   string `json:"health_url"`
	Status      string `json:"status"`
	Health      string `json:"health,omitempty"`
	Container   string `json:"container,omitempty"`
}

type ConsoleCandidateNode struct {
	NodeID       uint64 `json:"node_id"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	PodName      string `json:"pod_name"`
	PodIP        string `json:"pod_ip"`
	ServiceName  string `json:"service_name"`
	RaftAddress  string `json:"raft_address"`
	GRPCEndpoint string `json:"grpc_endpoint"`
	Phase        string `json:"phase"`
	Joined       bool   `json:"joined"`
	JoinEligible bool   `json:"join_eligible"`
	Reason       string `json:"reason,omitempty"`
}

type ConsoleClusterNodeAction struct {
	Action string `json:"action"`
}

type ConsoleClusterNodeActionResult struct {
	Node       string                       `json:"node"`
	Action     string                       `json:"action"`
	Status     string                       `json:"status"`
	Message    string                       `json:"message,omitempty"`
	RaftGroups []ConsoleRaftGroupJoinResult `json:"raft_groups,omitempty"`
}

type ConsoleRaftGroupJoinResult struct {
	ClusterID   uint64 `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

func registerConsoleRoutes(api *API, v1 *gin.RouterGroup) {
	if api == nil || api.component == nil || v1 == nil {
		return
	}
	comp := api.component
	if !comp.ConsoleEnabled {
		return
	}
	if comp.Console == nil {
		logger.Logger.Warn().Msg("console disabled: provider is nil")
		return
	}
	if comp.ConsoleUsername == "" || comp.ConsolePassword == "" {
		logger.Logger.Warn().Msg("console disabled: basic auth username/password is empty")
		return
	}

	auth := gin.BasicAuth(gin.Accounts{comp.ConsoleUsername: comp.ConsolePassword})
	web := api.httpServer.Group("/console")
	web.Use(auth)
	web.GET("", serveConsoleIndex)
	web.GET("/", serveConsoleIndex)
	web.GET("/assets/*filepath", serveConsoleAsset)
	api.httpServer.NoRoute(consoleFallbackRoute(comp.ConsoleUsername, comp.ConsolePassword))

	h := &consoleHandler{provider: comp.Console}
	consoleAPI := v1.Group("/console")
	consoleAPI.Use(auth)
	consoleAPI.GET("/summary", h.getSummary)
	consoleAPI.GET("/clients", h.listClients)
	consoleAPI.GET("/clients/:client_id", h.getClient)
	consoleAPI.GET("/subscriptions/tree", h.getSubscriptionTree)
	consoleAPI.GET("/share-groups/:share_group/members", h.getShareGroupMembers)
	consoleAPI.GET("/retain", h.listRetainMessages)
	consoleAPI.GET("/will-delay/due", h.listDueWillDelayTasks)
	consoleAPI.GET("/cluster/nodes", h.listClusterNodes)
	consoleAPI.POST("/cluster/nodes/:node/actions", h.runClusterNodeAction)
}

type consoleHandler struct {
	provider ConsoleProvider
}

func (h *consoleHandler) getSummary(c *gin.Context) {
	summary, err := h.provider.GetSummary(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, base.WithData(summary))
}

func (h *consoleHandler) listClients(c *gin.Context) {
	query := ConsoleClientQuery{
		ClientID: strings.TrimSpace(c.Query("client_id")),
		Limit:    parseConsoleLimit(c.Query("limit")),
	}
	clients, err := h.provider.ListClients(c.Request.Context(), query)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, base.WithData(clients))
}

func (h *consoleHandler) getClient(c *gin.Context) {
	clientID := strings.TrimSpace(c.Param("client_id"))
	if clientID == "" {
		writeBadRequest(c, errors.New("client_id is required"))
		return
	}
	detail, ok, err := h.provider.GetClient(c.Request.Context(), clientID)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeNotFound(c, "client not found")
		return
	}
	c.JSON(http.StatusOK, base.WithData(detail))
}

func (h *consoleHandler) getSubscriptionTree(c *gin.Context) {
	maxDepth := int32(parseConsoleLimit(c.Query("max_depth")))
	query := ConsoleSubscriptionTreeQuery{
		Topic:    strings.TrimSpace(c.Query("topic")),
		MaxDepth: maxDepth,
	}
	tree, err := h.provider.GetSubscriptionTree(c.Request.Context(), query)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, base.WithData(tree))
}

func (h *consoleHandler) getShareGroupMembers(c *gin.Context) {
	shareGroup := strings.TrimSpace(c.Param("share_group"))
	if shareGroup == "" {
		writeBadRequest(c, errors.New("share_group is required"))
		return
	}
	members, err := h.provider.GetShareGroupMembers(c.Request.Context(), shareGroup)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, base.WithData(members))
}

func (h *consoleHandler) listRetainMessages(c *gin.Context) {
	query := ConsoleRetainQuery{
		TopicFilter: strings.TrimSpace(c.Query("topic_filter")),
		Limit:       parseConsoleLimit(c.Query("limit")),
	}
	retained, err := h.provider.ListRetainMessages(c.Request.Context(), query)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, base.WithData(retained))
}

func (h *consoleHandler) listDueWillDelayTasks(c *gin.Context) {
	before := time.Now().UnixMicro()
	if raw := strings.TrimSpace(c.Query("before")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeBadRequest(c, err)
			return
		}
		before = parsed
	}
	query := ConsoleWillDelayQuery{
		BeforeUnixMicro: before,
		Limit:           parseConsoleLimit(c.Query("limit")),
	}
	tasks, err := h.provider.ListDueWillDelayTasks(c.Request.Context(), query)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, base.WithData(tasks))
}

func (h *consoleHandler) listClusterNodes(c *gin.Context) {
	nodes, err := h.provider.ListClusterNodes(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, base.WithData(nodes))
}

func (h *consoleHandler) runClusterNodeAction(c *gin.Context) {
	node := strings.TrimSpace(c.Param("node"))
	if node == "" {
		writeBadRequest(c, errors.New("node is required"))
		return
	}
	var req ConsoleClusterNodeAction
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, err)
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	switch req.Action {
	case "start", "stop", "restart", "join":
	default:
		writeBadRequest(c, errors.New("unsupported cluster node action"))
		return
	}
	result, err := h.provider.RunClusterNodeAction(c.Request.Context(), node, req)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, base.WithData(result))
}

func parseConsoleLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return defaultConsoleLimit
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func serveConsoleIndex(c *gin.Context) {
	path := filepath.Join("web", "console", "dist", "index.html")
	if _, err := os.Stat(path); err == nil {
		c.File(path)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(consoleFallbackHTML()))
}

func serveConsoleAsset(c *gin.Context) {
	rawPath := strings.TrimPrefix(c.Param("filepath"), "/")
	cleanPath := filepath.Clean(rawPath)
	if cleanPath == "." || strings.HasPrefix(cleanPath, "..") {
		c.Status(http.StatusNotFound)
		return
	}

	path := filepath.Join("web", "console", "dist", "assets", cleanPath)
	if _, err := os.Stat(path); err == nil {
		c.File(path)
		return
	}
	c.Status(http.StatusNotFound)
}

func consoleFallbackRoute(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/console/") {
			c.Status(http.StatusNotFound)
			return
		}
		if !validBasicAuth(c.Request, username, password) {
			c.Header("WWW-Authenticate", `Basic realm="Authorization Required"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		serveConsoleIndex(c)
	}
}

func validBasicAuth(req *http.Request, username, password string) bool {
	gotUsername, gotPassword, ok := req.BasicAuth()
	if !ok {
		return false
	}
	usernameOK := subtle.ConstantTimeCompare([]byte(gotUsername), []byte(username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(gotPassword), []byte(password)) == 1
	return usernameOK && passwordOK
}

func consoleFallbackHTML() string {
	return `<!doctype html><html><head><meta charset="utf-8"><title>SkyTree Console</title></head><body><div id="root">SkyTree Console assets are not built.</div></body></html>`
}
