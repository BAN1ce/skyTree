package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"strings"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/grpc/nodepb"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// RaftGRPCClient is a gRPC client that routes requests to a specific nodeID.
//
// This client previously used a single connection with round_robin load balancing
// (via a custom resolver), which cannot guarantee requests are delivered to the intended node.
// This implementation maintains a per-node connection pool keyed by nodeID to ensure point-to-point delivery.
type RaftGRPCClient struct {
	localNodeID uint64
	state       cluster.State

	mu               sync.RWMutex
	conns            map[uint64]*grpc.ClientConn
	lastError        map[uint64]error
	dialLocks        map[uint64]*sync.Mutex
	tlsConfig        config.TLS
	allowInsecure    bool
	transportCred    credentials.TransportCredentials
	credentialErr    error
	rpcTimeout       time.Duration
	preflightTimeout time.Duration
	traffic          cluster.TrafficStateStore
	connStates       map[uint64]connectivity.State
}

const ServiceCloseClientName = "ServiceCloseClient"
const defaultRaftGRPCRPCTimeout = 3 * time.Second
const defaultRaftGRPCPreflightTimeout = 500 * time.Millisecond
const (
	grpcServiceClientDeliveryNotify = "ClientDeliveryNotify"
	grpcServiceClientCenter         = "ClientCenter"
	grpcMethodNotifyClientDelivery  = "NotifyClientDelivery"
	grpcMethodSharedWake            = "NotifySharedSubscriptionWake"
	grpcMethodCloseClient           = "CloseClient"
)

// NewRaftGRPCClient 创建Raft gRPC客户端
func NewRaftGRPCClient(localNodeID uint64, state cluster.State, tlsCfg config.TLS, allowInsecure bool) *RaftGRPCClient {
	c := &RaftGRPCClient{
		localNodeID:      localNodeID,
		state:            state,
		conns:            make(map[uint64]*grpc.ClientConn, 16),
		lastError:        make(map[uint64]error, 16),
		dialLocks:        make(map[uint64]*sync.Mutex, 16),
		tlsConfig:        tlsCfg,
		allowInsecure:    allowInsecure,
		transportCred:    nil,
		credentialErr:    nil,
		rpcTimeout:       defaultRaftGRPCRPCTimeout,
		preflightTimeout: defaultRaftGRPCPreflightTimeout,
		connStates:       make(map[uint64]connectivity.State, 16),
	}
	c.transportCred, c.credentialErr = c.buildTransportCredentials()
	return c
}

func (c *RaftGRPCClient) SetTrafficStateStore(store cluster.TrafficStateStore) {
	if c == nil {
		return
	}
	c.traffic = store
}

// Start 启动客户端
func (c *RaftGRPCClient) Start(ctx context.Context) error {
	if c.credentialErr != nil {
		return c.credentialErr
	}
	logger.Logger.Info().
		Bool("tls_enabled", c.tlsConfig.Enabled).
		Bool("allow_insecure", c.allowInsecure).
		Msg("gRPC client started (point-to-point per nodeID)")
	// 阻塞等待 context 取消
	<-ctx.Done()
	return nil
}

func (c *RaftGRPCClient) dialOptions() ([]grpc.DialOption, error) {
	if c.credentialErr != nil {
		return nil, c.credentialErr
	}
	return []grpc.DialOption{
		grpc.WithTransportCredentials(c.transportCred),
		grpc.WithNoProxy(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}, nil
}

func (c *RaftGRPCClient) getDialLock(nodeID uint64) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l, ok := c.dialLocks[nodeID]; ok {
		return l
	}
	l := &sync.Mutex{}
	c.dialLocks[nodeID] = l
	return l
}

func (c *RaftGRPCClient) getNodeEndpoint(ctx context.Context, nodeID uint64) (string, error) {
	nodes, err := c.state.ListNode(ctx)
	if err != nil {
		return "", err
	}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.LocalNodeID != nodeID {
			continue
		}
		// Prefer Endpoint (reachable address), not Addr (often 0.0.0.0 bind address).
		if n.GRPC.Endpoint != "" {
			return n.GRPC.Endpoint, nil
		}
		if n.GRPC.Addr != "" {
			return n.GRPC.Addr, nil
		}
		return "", fmt.Errorf("node %d has empty grpc endpoint", nodeID)
	}
	return "", fmt.Errorf("node %d not found", nodeID)
}

func (c *RaftGRPCClient) getConn(ctx context.Context, nodeID uint64) (*grpc.ClientConn, error) {
	return c.getConnWithCloseClientMetrics(ctx, nodeID, false)
}

func (c *RaftGRPCClient) getConnForCloseClient(ctx context.Context, nodeID uint64) (*grpc.ClientConn, error) {
	return c.getConnWithCloseClientMetrics(ctx, nodeID, true)
}

func (c *RaftGRPCClient) getConnWithCloseClientMetrics(ctx context.Context, nodeID uint64, recordCloseClientMetrics bool) (_ *grpc.ClientConn, err error) {
	var startTime time.Time
	if recordCloseClientMetrics {
		startTime = time.Now()
		defer func() {
			metric.RecordCloseClientClientStage("get_conn_total", closeClientStageResultFromError(err), time.Since(startTime))
			if err != nil {
				metric.RecordCloseClientClientFailure("get_conn_total", closeClientFailureReasonFromError(err, true))
			}
		}()
	}

	c.mu.RLock()
	conn, ok := c.conns[nodeID]
	c.mu.RUnlock()
	if ok && conn != nil {
		if reusableErr := c.ensureConnReusable(ctx, nodeID, conn); reusableErr == nil {
			metric.SetGRPCClientConnection(nodeID, "connected", 1)
			if recordCloseClientMetrics {
				metric.RecordCloseClientClientPath("conn_cache_hit", "success")
			}
			return conn, nil
		}
	}
	if recordCloseClientMetrics {
		metric.RecordCloseClientClientPath("conn_cache_miss", "success")
	}

	lock := c.getDialLock(nodeID)
	lock.Lock()
	defer lock.Unlock()

	// Double-check after acquiring per-node lock.
	c.mu.RLock()
	conn, ok = c.conns[nodeID]
	c.mu.RUnlock()
	if ok && conn != nil {
		if reusableErr := c.ensureConnReusable(ctx, nodeID, conn); reusableErr == nil {
			metric.SetGRPCClientConnection(nodeID, "connected", 1)
			if recordCloseClientMetrics {
				metric.RecordCloseClientClientPath("conn_cache_hit", "success")
			}
			return conn, nil
		}
	}

	endpointStart := time.Now()
	endpoint, err := c.getNodeEndpoint(ctx, nodeID)
	if recordCloseClientMetrics {
		metric.RecordCloseClientClientStage("endpoint_lookup", closeClientStageResultFromError(err), time.Since(endpointStart))
		if err != nil {
			metric.RecordCloseClientClientFailure("endpoint_lookup", closeClientFailureReasonFromError(err, true))
		}
	}
	if err != nil {
		c.mu.Lock()
		c.lastError[nodeID] = err
		c.mu.Unlock()
		metric.SetGRPCClientConnection(nodeID, "error", 1)
		if recordCloseClientMetrics {
			metric.RecordCloseClientClientPath("dial_new_conn", closeClientStageResultFromError(err))
		}
		return nil, err
	}

	logger.Logger.Debug().Uint64("nodeID", nodeID).Str("endpoint", endpoint).Msg("dialing gRPC node endpoint")
	options, err := c.dialOptions()
	if err != nil {
		c.mu.Lock()
		c.lastError[nodeID] = err
		c.mu.Unlock()
		metric.SetGRPCClientConnection(nodeID, "error", 1)
		if recordCloseClientMetrics {
			metric.RecordCloseClientClientPath("dial_new_conn", closeClientStageResultFromError(err))
		}
		return nil, err
	}
	conn, err = grpc.NewClient(endpoint, options...)
	if err != nil {
		c.mu.Lock()
		c.lastError[nodeID] = err
		c.mu.Unlock()
		metric.SetGRPCClientConnection(nodeID, "error", 1)
		if recordCloseClientMetrics {
			metric.RecordCloseClientClientPath("dial_new_conn", closeClientStageResultFromError(err))
		}
		return nil, err
	}

	c.mu.Lock()
	c.conns[nodeID] = conn
	delete(c.lastError, nodeID)
	c.mu.Unlock()
	c.recordConnState(nodeID, conn.GetState())

	metric.SetGRPCClientConnection(nodeID, "connected", 1)
	metric.SetGRPCClientConnection(nodeID, "error", 0)
	if recordCloseClientMetrics {
		metric.RecordCloseClientClientPath("dial_new_conn", "success")
	}
	if err := c.preflightConn(ctx, nodeID, conn); err != nil {
		c.dropConn(nodeID, err)
		return nil, err
	}
	return conn, nil
}

func (c *RaftGRPCClient) buildTransportCredentials() (credentials.TransportCredentials, error) {
	if c.tlsConfig.Enabled {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		if strings.TrimSpace(c.tlsConfig.CAFile) != "" {
			pemData, err := os.ReadFile(c.tlsConfig.CAFile)
			if err != nil {
				return nil, fmt.Errorf("read cluster.grpc.tls.ca_file failed: %w", err)
			}
			pool := x509.NewCertPool()
			if ok := pool.AppendCertsFromPEM(pemData); !ok {
				return nil, fmt.Errorf("parse cluster.grpc.tls.ca_file failed")
			}
			tlsCfg.RootCAs = pool
		}

		certFile := strings.TrimSpace(c.tlsConfig.CertFile)
		keyFile := strings.TrimSpace(c.tlsConfig.KeyFile)
		if certFile != "" || keyFile != "" {
			if certFile == "" || keyFile == "" {
				return nil, fmt.Errorf("cluster.grpc.tls.cert_file and key_file must be both set for mTLS client cert")
			}
			certPair, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, fmt.Errorf("load cluster.grpc.tls client cert/key failed: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{certPair}
		}

		return credentials.NewTLS(tlsCfg), nil
	}

	if c.allowInsecure {
		if logger.Logger != nil {
			logger.Logger.Warn().Msg("cluster gRPC client is running without TLS (allow_insecure=true)")
		}
		return insecure.NewCredentials(), nil
	}

	return nil, fmt.Errorf("cluster gRPC client TLS is disabled: set cluster.grpc.tls.enabled=true or cluster.grpc.allow_insecure=true")
}

func (c *RaftGRPCClient) dropConn(nodeID uint64, err error) {
	c.mu.Lock()
	conn := c.conns[nodeID]
	delete(c.conns, nodeID)
	delete(c.connStates, nodeID)
	c.lastError[nodeID] = err
	c.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	metric.SetGRPCClientConnection(nodeID, "connected", 0)
	metric.SetGRPCClientConnection(nodeID, "error", 1)
}

func (c *RaftGRPCClient) withRPCTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := c.rpcTimeout
	if timeout <= 0 {
		timeout = defaultRaftGRPCRPCTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *RaftGRPCClient) canRouteToNode(nodeID uint64) error {
	if c == nil || c.traffic == nil {
		return nil
	}
	if c.traffic.CanRouteToNode(nodeID) {
		return nil
	}
	reason := "node_not_ready"
	switch c.traffic.NodeTrafficState(nodeID) {
	case cluster.TrafficStateSuspect:
		reason = "node_suspect"
	case cluster.TrafficStateDraining:
		reason = "node_draining"
	}
	metric.RecordPeerRPCFastFail(nodeID, reason)
	return status.Error(codes.Unavailable, "peer node is not traffic-ready")
}

func (c *RaftGRPCClient) ensureConnReusable(ctx context.Context, nodeID uint64, conn *grpc.ClientConn) error {
	if conn == nil {
		return status.Error(codes.Unavailable, "grpc connection is nil")
	}
	state := conn.GetState()
	c.recordConnState(nodeID, state)
	switch state {
	case connectivity.Ready:
		return nil
	case connectivity.Idle, connectivity.Connecting:
		return c.preflightConn(ctx, nodeID, conn)
	case connectivity.TransientFailure:
		metric.RecordPeerConnEviction(nodeID, "transient_failure")
		c.dropConn(nodeID, status.Error(codes.Unavailable, "grpc connection in transient failure"))
		return status.Error(codes.Unavailable, "grpc connection in transient failure")
	case connectivity.Shutdown:
		metric.RecordPeerConnEviction(nodeID, "shutdown")
		c.dropConn(nodeID, status.Error(codes.Unavailable, "grpc connection is shutdown"))
		return status.Error(codes.Unavailable, "grpc connection is shutdown")
	default:
		return c.preflightConn(ctx, nodeID, conn)
	}
}

func (c *RaftGRPCClient) preflightConn(ctx context.Context, nodeID uint64, conn *grpc.ClientConn) error {
	start := time.Now()
	if conn == nil {
		err := status.Error(codes.Unavailable, "grpc connection is nil")
		metric.RecordPeerPreflight(nodeID, "error", time.Since(start))
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	preflightCtx := ctx
	cancel := func() {}
	if deadline, hasDeadline := ctx.Deadline(); !hasDeadline || time.Until(deadline) > c.preflightTimeout {
		preflightCtx, cancel = context.WithTimeout(ctx, c.preflightTimeout)
	}
	defer cancel()

	conn.Connect()
	resp, err := grpc_health_v1.NewHealthClient(conn).Check(preflightCtx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		metric.RecordPeerPreflight(nodeID, closeClientStageResultFromError(err), time.Since(start))
		if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
			metric.RecordPeerConnEviction(nodeID, "connecting_timeout")
		}
		return err
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		metric.RecordPeerPreflight(nodeID, "error", time.Since(start))
		return status.Error(codes.Unavailable, "grpc health is not serving")
	}
	c.recordConnState(nodeID, conn.GetState())
	metric.RecordPeerPreflight(nodeID, "success", time.Since(start))
	return nil
}

func (c *RaftGRPCClient) recordConnState(nodeID uint64, state connectivity.State) {
	if c == nil {
		return
	}
	to := connectivityStateLabel(state)
	c.mu.Lock()
	from := connectivityStateLabel(c.connStates[nodeID])
	c.connStates[nodeID] = state
	c.mu.Unlock()
	metric.RecordPeerConnStateTransition(nodeID, from, to)
}

func connectivityStateLabel(state connectivity.State) string {
	switch state {
	case connectivity.Idle:
		return "idle"
	case connectivity.Connecting:
		return "connecting"
	case connectivity.Ready:
		return "ready"
	case connectivity.TransientFailure:
		return "transient_failure"
	case connectivity.Shutdown:
		return "shutdown"
	default:
		return "unknown"
	}
}

func (c *RaftGRPCClient) markPeerSuspect(nodeID uint64, err error) {
	controller, ok := c.traffic.(cluster.TrafficController)
	if !ok || controller == nil {
		return
	}
	controller.SetNodeState(nodeID, cluster.TrafficStateSuspect, closeClientFailureReasonFromError(err, true))
	metric.SetNodeTrafficState(nodeID, cluster.TrafficStateSuspect)
}

func (c *RaftGRPCClient) callClientDeliveryNotify(
	ctx context.Context,
	nodeID uint64,
	method string,
	req *nodepb.ClientDeliveryNotifyRequest,
) (err error) {
	startTime := time.Now()
	finishInflight := metric.BeginGRPCClientRequest(grpcServiceClientDeliveryNotify, method, nodeID)
	requestBytes := proto.Size(req)
	responseBytes := 0
	defer func() {
		finishInflight()
		metric.RecordGRPCClientRequest(
			grpcServiceClientDeliveryNotify,
			method,
			nodeID,
			status.Code(err),
			err,
			time.Since(startTime),
			requestBytes,
			responseBytes,
		)
	}()

	conn, err := c.getConn(ctx, nodeID)
	if err != nil {
		logger.Logger.Error().Err(err).Uint64("nodeID", nodeID).Msg("failed to get gRPC connection")
		return err
	}
	client := nodepb.NewClientDeliveryNotifyClient(conn)
	res, err := client.Notify(ctx, req)
	if res != nil {
		responseBytes = proto.Size(res)
	}
	return err
}

// NotifyClientDelivery notifies a target node to wake client delivery runners or deliver QoS0 directly.
func (c *RaftGRPCClient) NotifyClientDelivery(ctx context.Context, nodeID uint64, publishTopic string, clientIDs []string, kind int32, payload []byte, clientOptions map[string]cluster.ClientDeliveryOptions) error {
	if nodeID == c.localNodeID {
		return nil
	}
	if len(clientIDs) == 0 {
		return nil
	}

	rpcCtx, cancel := c.withRPCTimeout(ctx)
	defer cancel()
	if err := c.canRouteToNode(nodeID); err != nil {
		return err
	}

	// Convert clientOptions to proto format.
	protoClientOptions := make(map[string]*nodepb.ClientDeliveryOptions, len(clientOptions))
	for clientID, opts := range clientOptions {
		protoClientOptions[clientID] = &nodepb.ClientDeliveryOptions{
			NoLocal:             opts.NoLocal,
			RAP:                 opts.RAP,
			SubscriptionIDsJSON: opts.SubscriptionIDsJSON,
		}
	}

	req := &nodepb.ClientDeliveryNotifyRequest{
		TargetNodeID:  nodeID,
		ClientIDs:     clientIDs,
		Kind:          nodepb.ClientDeliveryNotifyKind(kind),
		Payload:       payload,
		PublishTopic:  publishTopic,
		ClientOptions: protoClientOptions,
	}
	err := c.callClientDeliveryNotify(
		rpcCtx,
		nodeID,
		grpcMethodNotifyClientDelivery,
		req,
	)
	if err != nil {
		// Drop the per-node connection on error so the next request will redial.
		logger.Logger.Warn().Err(err).Uint64("nodeID", nodeID).Msg("gRPC call failed, dropping connection")
		c.markPeerSuspect(nodeID, err)
		switch status.Code(err) {
		case codes.Unavailable:
			metric.RecordPeerConnEviction(nodeID, "rpc_unavailable")
		case codes.DeadlineExceeded:
			metric.RecordPeerConnEviction(nodeID, "rpc_timeout")
		default:
			metric.RecordPeerConnEviction(nodeID, "rpc_error")
		}
		c.dropConn(nodeID, err)
	}
	return err
}

// NotifySharedSubscriptionWake broadcasts a shared subscription wake to every peer broker node.
func (c *RaftGRPCClient) NotifySharedSubscriptionWake(ctx context.Context, wake cluster.SharedSubscriptionWake) error {
	if c == nil || c.state == nil {
		return nil
	}
	if wake.ShareGroup == "" || wake.TopicFilter == "" || wake.TaskID == "" {
		return fmt.Errorf("shared subscription wake is incomplete")
	}
	payload, err := json.Marshal(wake)
	if err != nil {
		return err
	}
	nodes, err := c.state.ListNode(ctx)
	if err != nil {
		return err
	}
	var notifyErrs []error
	for _, node := range nodes {
		if node == nil || node.LocalNodeID == 0 || node.LocalNodeID == c.localNodeID {
			continue
		}
		if err := c.notifySharedSubscriptionWakeToNode(ctx, uint64(node.LocalNodeID), payload); err != nil {
			notifyErrs = append(notifyErrs, fmt.Errorf("node %d: %w", node.LocalNodeID, err))
		}
	}
	return errors.Join(notifyErrs...)
}

func (c *RaftGRPCClient) notifySharedSubscriptionWakeToNode(ctx context.Context, nodeID uint64, payload []byte) error {
	rpcCtx, cancel := c.withRPCTimeout(ctx)
	defer cancel()
	if err := c.canRouteToNode(nodeID); err != nil {
		return err
	}

	req := &nodepb.ClientDeliveryNotifyRequest{
		TargetNodeID: nodeID,
		Kind:         nodepb.ClientDeliveryNotifyKind_CLIENT_DELIVERY_NOTIFY_KIND_SHARED_WAKE,
		Payload:      payload,
	}
	err := c.callClientDeliveryNotify(rpcCtx, nodeID, grpcMethodSharedWake, req)
	if err != nil {
		logger.Logger.Warn().Err(err).Uint64("nodeID", nodeID).Msg("shared subscription wake gRPC call failed, dropping connection")
		c.markPeerSuspect(nodeID, err)
		switch status.Code(err) {
		case codes.Unavailable:
			metric.RecordPeerConnEviction(nodeID, "rpc_unavailable")
		case codes.DeadlineExceeded:
			metric.RecordPeerConnEviction(nodeID, "rpc_timeout")
		default:
			metric.RecordPeerConnEviction(nodeID, "rpc_error")
		}
		c.dropConn(nodeID, err)
	}
	return err
}

// RequestCloseClient 请求关闭客户端
func (c *RaftGRPCClient) RequestCloseClient(ctx context.Context, nodeID uint64, clientID string, ownerToken string) (err error) {
	totalStart := time.Now()
	if nodeID == c.localNodeID {
		metric.RecordCloseClientClientPath("local_skip", "success")
		metric.RecordCloseClientClientStage("total", "success", time.Since(totalStart))
		return nil
	}

	startTime := time.Now()
	finishInflight := metric.BeginGRPCClientRequest(grpcServiceClientCenter, grpcMethodCloseClient, nodeID)
	requestBytes := 0
	responseBytes := 0
	defer func() {
		finishInflight()
		metric.RecordGRPCClientRequest(
			grpcServiceClientCenter,
			grpcMethodCloseClient,
			nodeID,
			status.Code(err),
			err,
			time.Since(startTime),
			requestBytes,
			responseBytes,
		)
		metric.RecordCloseClientClientStage("total", closeClientStageResultFromError(err), time.Since(totalStart))
	}()

	rpcCtx, cancel := c.withRPCTimeout(ctx)
	defer cancel()
	if routeErr := c.canRouteToNode(nodeID); routeErr != nil {
		err = routeErr
		return err
	}

	conn, err := c.getConnForCloseClient(rpcCtx, nodeID)
	if err != nil {
		logger.Logger.Error().Err(err).Uint64("nodeID", nodeID).Msg("failed to get gRPC connection")
		return err
	}

	client := nodepb.NewClientCenterClient(conn)
	req := &nodepb.CloseClientRequest{
		ClientID:   clientID,
		OwnerToken: ownerToken,
	}
	requestBytes = proto.Size(req)
	invokeStart := time.Now()
	res, err := client.CloseClient(rpcCtx, req)
	metric.RecordCloseClientClientStage("rpc_invoke", closeClientStageResultFromError(err), time.Since(invokeStart))

	if err != nil {
		metric.RecordCloseClientClientFailure("rpc_invoke", closeClientFailureReasonFromError(err, false))
		// Drop the per-node connection on error so the next request will redial.
		logger.Logger.Warn().Err(err).Uint64("nodeID", nodeID).Msg("gRPC call failed, dropping connection")
		metric.RecordCloseClientClientPath("drop_conn", closeClientStageResultFromError(err))
		c.markPeerSuspect(nodeID, err)
		switch status.Code(err) {
		case codes.Unavailable:
			metric.RecordPeerConnEviction(nodeID, "rpc_unavailable")
		case codes.DeadlineExceeded:
			metric.RecordPeerConnEviction(nodeID, "rpc_timeout")
		default:
			metric.RecordPeerConnEviction(nodeID, "rpc_error")
		}
		c.dropConn(nodeID, err)
		return err
	}
	responseBytes = proto.Size(res)

	if !res.GetSuccess() {
		metric.RecordCloseClientClientFailure("response_validate", closeClientFailureReasonFromResponse(res))
		return fmt.Errorf("close client failed: %s", res.GetMessage())
	}

	return nil
}

func closeClientStageResultFromError(err error) string {
	code := status.Code(err)
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.DeadlineExceeded), code == codes.DeadlineExceeded:
		return "timeout"
	case errors.Is(err, context.Canceled), code == codes.Canceled:
		return "canceled"
	default:
		return "error"
	}
}

func closeClientFailureReasonFromError(err error, fallbackToConnection bool) string {
	code := status.Code(err)
	if errors.Is(err, context.DeadlineExceeded) || code == codes.DeadlineExceeded {
		return "deadline_exceeded"
	}
	if errors.Is(err, context.Canceled) || code == codes.Canceled {
		return "canceled"
	}

	switch code {
	case codes.Unavailable:
		return "unavailable"
	case codes.Unknown:
		return "unknown"
	}

	if fallbackToConnection {
		return "connection_error"
	}
	return "unknown"
}

func closeClientFailureReasonFromResponse(res *nodepb.CloseClientResponse) string {
	if res == nil {
		return "unexpected_response"
	}
	switch res.GetMessage() {
	case "client not found":
		return "not_found"
	case "owner token mismatch":
		return "owner_conflict"
	default:
		return "unexpected_response"
	}
}

// Name 返回组件名称
func (c *RaftGRPCClient) Name() string {
	return "raft-grpc-client"
}

// IsConnected 检查连接状态
func (c *RaftGRPCClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.conns) != 0
}

// GetConnectionStatus 获取连接状态信息
func (c *RaftGRPCClient) GetConnectionStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"connections": len(c.conns),
		"last_error":  c.lastError,
	}
}

// ResetConnection 重置连接状态，强制下次重新连接
func (c *RaftGRPCClient) ResetConnection() {
	c.mu.Lock()
	conns := make([]*grpc.ClientConn, 0, len(c.conns))
	nodeIDs := make([]uint64, 0, len(c.conns))
	for nodeID, conn := range c.conns {
		nodeIDs = append(nodeIDs, nodeID)
		if conn != nil {
			conns = append(conns, conn)
		}
	}
	c.conns = make(map[uint64]*grpc.ClientConn, 16)
	c.lastError = make(map[uint64]error, 16)
	c.connStates = make(map[uint64]connectivity.State, 16)
	c.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
	for _, nodeID := range nodeIDs {
		metric.SetGRPCClientConnection(nodeID, "connected", 0)
	}

	logger.Logger.Info().Msg("gRPC connections reset")
}

// Close 关闭客户端
func (c *RaftGRPCClient) Close() error {
	c.ResetConnection()
	return nil
}
