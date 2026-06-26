package api

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/BAN1ce/skyTree/api/base"
	inner_cluster "github.com/BAN1ce/skyTree/internal/clusterhealth"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/gin-gonic/gin"
)

type clusterHealthItem struct {
	ClusterID    uint64    `json:"cluster_id"`
	ClusterName  string    `json:"cluster_name"`
	Status       string    `json:"status"`
	FailureCount int       `json:"failure_count"`
	LastCheck    time.Time `json:"last_check"`
	LastSuccess  time.Time `json:"last_success"`
	LatencyMs    int64     `json:"latency_ms"`
	Error        string    `json:"error,omitempty"`
}

type clusterHealthSummary struct {
	Timestamp time.Time           `json:"timestamp"`
	Total     int                 `json:"total"`
	Healthy   int                 `json:"healthy"`
	Unhealthy int                 `json:"unhealthy"`
	Unknown   int                 `json:"unknown"`
	Clusters  []clusterHealthItem `json:"clusters"`
}

type ClusterOverview struct {
	Timestamp       time.Time                      `json:"timestamp"`
	ClusterEnabled  bool                           `json:"cluster_enabled"`
	RaftGroups      []ClusterRaftGroupOverview     `json:"raft_groups"`
	DeliveryBacklog ClusterDeliveryBacklogOverview `json:"delivery_backlog"`
}

type ClusterRaftGroupOverview struct {
	ClusterID         uint64 `json:"cluster_id"`
	ClusterName       string `json:"cluster_name"`
	Kind              string `json:"kind"`
	ShardID           *int   `json:"shard_id,omitempty"`
	HasLeader         bool   `json:"has_leader"`
	LeaderNodeID      uint64 `json:"leader_node_id,omitempty"`
	Health            string `json:"health"`
	ReplicationStatus string `json:"replication_status"`
}

type ClusterDeliveryBacklogOverview struct {
	Supported     bool   `json:"supported"`
	Reason        string `json:"reason,omitempty"`
	PendingTasks  int64  `json:"pending_tasks"`
	ActiveClients int64  `json:"active_clients"`
}

func registerClusterRoutes(v1 *gin.RouterGroup, comp *Component) {
	if v1 == nil {
		return
	}
	if comp == nil {
		return
	}

	if comp.ClusterHealth == nil && comp.ClusterOverview == nil {
		logger.Logger.Warn().Msg("cluster API disabled: no provider")
		return
	}

	h := &clusterHandler{
		provider:         comp.ClusterHealth,
		overviewProvider: comp.ClusterOverview,
	}
	group := v1.Group("/cluster")
	if h.provider != nil {
		group.GET("/health", h.getAll)
		group.GET("/health/:cluster_id", h.getOne)
	} else {
		logger.Logger.Warn().Msg("cluster health API disabled: health checker is nil")
	}
	if h.overviewProvider != nil {
		group.GET("/overview", h.getOverview)
	} else {
		logger.Logger.Warn().Msg("cluster overview API disabled: overview provider is nil")
	}
}

type clusterHandler struct {
	provider         ClusterHealthProvider
	overviewProvider ClusterOverviewProvider
}

func (h *clusterHandler) getAll(c *gin.Context) {
	statuses := h.provider.GetAllHealthStatus()
	items := make([]clusterHealthItem, 0, len(statuses))
	summary := clusterHealthSummary{Timestamp: time.Now(), Total: len(statuses)}

	for _, st := range statuses {
		if st == nil {
			continue
		}
		statusText := healthStatusText(st.Status)
		switch statusText {
		case "healthy":
			summary.Healthy++
		case "unhealthy":
			summary.Unhealthy++
		default:
			summary.Unknown++
		}
		items = append(items, clusterHealthItem{
			ClusterID:    st.ClusterID,
			ClusterName:  st.ClusterName,
			Status:       statusText,
			FailureCount: st.FailureCount,
			LastCheck:    st.LastCheck,
			LastSuccess:  st.LastSuccess,
			LatencyMs:    st.Latency.Milliseconds(),
			Error:        st.Error,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ClusterID < items[j].ClusterID
	})
	summary.Clusters = items

	c.JSON(http.StatusOK, base.WithData(summary))
}

func (h *clusterHandler) getOne(c *gin.Context) {
	rawID := c.Param("cluster_id")
	clusterID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		writeBadRequest(c, err)
		return
	}

	status, ok := h.provider.GetHealthStatus(clusterID)
	if !ok || status == nil {
		writeNotFound(c, "cluster not found")
		return
	}

	c.JSON(http.StatusOK, base.WithData(clusterHealthItem{
		ClusterID:    status.ClusterID,
		ClusterName:  status.ClusterName,
		Status:       healthStatusText(status.Status),
		FailureCount: status.FailureCount,
		LastCheck:    status.LastCheck,
		LastSuccess:  status.LastSuccess,
		LatencyMs:    status.Latency.Milliseconds(),
		Error:        status.Error,
	}))
}

func (h *clusterHandler) getOverview(c *gin.Context) {
	if h.overviewProvider == nil {
		writeNotFound(c, "cluster overview provider is unavailable")
		return
	}

	overview, err := h.overviewProvider.GetClusterOverview(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	if overview == nil {
		c.JSON(http.StatusOK, base.WithData(ClusterOverview{
			Timestamp: time.Now(),
		}))
		return
	}

	c.JSON(http.StatusOK, base.WithData(overview))
}

func healthStatusText(s inner_cluster.HealthStatus) string {
	switch s {
	case inner_cluster.HealthStatusHealthy:
		return "healthy"
	case inner_cluster.HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}
