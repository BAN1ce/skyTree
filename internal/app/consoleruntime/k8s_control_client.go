package consoleruntime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/api"
	"github.com/BAN1ce/skyTree/config"
	cluster_pkg "github.com/BAN1ce/skyTree/pkg/cluster"
	raft_pkg "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultK8sAPIServer = "https://kubernetes.default.svc"
)

type K8sControlDependencies struct {
	Control      config.ConsoleControl
	BaseCluster  config.Cluster
	ClusterState cluster_pkg.State
	Membership   membershipJoiner
	Traffic      cluster_pkg.TrafficController
	HTTPClient   *http.Client
}

type membershipJoiner interface {
	AddNode(ctx context.Context, nodeID uint64, target string) (*raft_pkg.MembershipResult, error)
}

type K8sControlClient struct {
	cfg                        config.ConsoleK8sControl
	baseCluster                config.Cluster
	clusterState               cluster_pkg.State
	membership                 membershipJoiner
	traffic                    cluster_pkg.TrafficController
	httpClient                 *http.Client
	activationProber           activationProber
	activationWarmup           time.Duration
	activationRetryInterval    time.Duration
	activationSuccessThreshold int
	listPodsFn                 func(context.Context) ([]k8sPod, error)
}

type activationProber interface {
	ProbeGRPC(ctx context.Context, endpoint string, tlsCfg config.TLS, allowInsecure bool) error
	ProbeHealth(ctx context.Context, healthURL string) error
	ProbeMQTT(ctx context.Context, address string) error
}

type defaultActivationProber struct {
	httpClient *http.Client
}

type k8sPodList struct {
	Items []k8sPod `json:"items"`
}

type k8sPod struct {
	Metadata k8sObjectMeta `json:"metadata"`
	Status   k8sPodStatus  `json:"status"`
}

type k8sObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type k8sPodStatus struct {
	Phase      string            `json:"phase"`
	PodIP      string            `json:"podIP"`
	Conditions []k8sPodCondition `json:"conditions"`
}

type k8sPodCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

func NewK8sControlClient(deps K8sControlDependencies) *K8sControlClient {
	cfg := k8sConfigWithDefaults(deps.Control.K8s)
	httpClient := deps.HTTPClient
	if httpClient == nil {
		httpClient = newK8sHTTPClient(cfg)
	}
	return &K8sControlClient{
		cfg:                        cfg,
		baseCluster:                deps.BaseCluster,
		clusterState:               deps.ClusterState,
		membership:                 deps.Membership,
		traffic:                    deps.Traffic,
		httpClient:                 httpClient,
		activationProber:           &defaultActivationProber{httpClient: httpClient},
		activationWarmup:           200 * time.Millisecond,
		activationRetryInterval:    500 * time.Millisecond,
		activationSuccessThreshold: 2,
	}
}

func (c *K8sControlClient) ListClusterNodes(ctx context.Context) ([]api.ConsoleRuntimeNode, error) {
	pods, err := c.listPods(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make([]api.ConsoleRuntimeNode, 0, len(pods))
	for _, pod := range pods {
		nodes = append(nodes, c.runtimeNode(pod))
	}
	return nodes, nil
}

func (c *K8sControlClient) ListCandidateNodes(ctx context.Context) ([]api.ConsoleCandidateNode, error) {
	pods, err := c.listPods(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make([]api.ConsoleCandidateNode, 0, len(pods))
	for _, pod := range pods {
		nodes = append(nodes, c.candidateNode(pod))
	}
	return nodes, nil
}

func (c *K8sControlClient) RunClusterNodeAction(
	ctx context.Context,
	node string,
	action string,
) (*api.ConsoleClusterNodeActionResult, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "join" {
		return nil, fmt.Errorf("unsupported k8s cluster node action %q", action)
	}
	candidate, ok, err := c.findCandidate(ctx, node)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("candidate node not found")
	}
	if !candidate.JoinEligible {
		return nil, fmt.Errorf("candidate node is not join eligible")
	}
	if c.membership == nil {
		return nil, errors.New("raft membership manager is unavailable")
	}
	if c.clusterState == nil {
		return nil, errors.New("cluster state is unavailable")
	}

	result, err := c.membership.AddNode(ctx, candidate.NodeID, candidate.RaftAddress)
	actionResult := membershipActionResult(candidate, result, err)
	if err != nil {
		return actionResult, err
	}
	if err := c.clusterState.AddNode(ctx, c.nodeMeta(candidate)); err != nil {
		return nil, fmt.Errorf("store joined cluster node metadata: %w", err)
	}
	c.startActivation(candidate)
	return actionResult, nil
}

func (c *K8sControlClient) findCandidate(
	ctx context.Context,
	node string,
) (api.ConsoleCandidateNode, bool, error) {
	node = strings.TrimSpace(node)
	if node == "" {
		return api.ConsoleCandidateNode{}, false, nil
	}
	nodes, err := c.ListCandidateNodes(ctx)
	if err != nil {
		return api.ConsoleCandidateNode{}, false, err
	}
	for _, candidate := range nodes {
		if candidateMatches(candidate, node) {
			return candidate, true, nil
		}
	}
	return api.ConsoleCandidateNode{}, false, nil
}

func (c *K8sControlClient) listPods(ctx context.Context) ([]k8sPod, error) {
	if c != nil && c.listPodsFn != nil {
		return c.listPodsFn(ctx)
	}
	if c == nil {
		return nil, errors.New("k8s control client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	endpoint, err := c.podListURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create k8s pod list request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	token, err := c.bearerToken()
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call k8s pod list api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("k8s pod list api returned status %d", resp.StatusCode)
	}
	var out k8sPodList
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode k8s pod list response: %w", err)
	}
	return out.Items, nil
}

func (c *K8sControlClient) startActivation(candidate api.ConsoleCandidateNode) {
	if c == nil || c.traffic == nil || candidate.NodeID == 0 {
		return
	}
	c.traffic.SetNodeState(candidate.NodeID, cluster_pkg.TrafficStateJoining, "activation_pending")
	metric.SetNodeTrafficState(candidate.NodeID, cluster_pkg.TrafficStateJoining)
	metric.RecordNodeActivationAttempt(candidate.NodeID)
	go c.activateNode(candidate)
}

func (c *K8sControlClient) activateNode(candidate api.ConsoleCandidateNode) {
	if c == nil || c.activationProber == nil || c.traffic == nil {
		return
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.JoinTimeout)
	defer cancel()

	successes := 0
	for {
		if ctx.Err() != nil {
			c.traffic.SetNodeState(candidate.NodeID, cluster_pkg.TrafficStateSuspect, "timeout")
			metric.SetNodeTrafficState(candidate.NodeID, cluster_pkg.TrafficStateSuspect)
			metric.RecordNodeActivationFailure(candidate.NodeID, "timeout")
			metric.RecordNodeActivationDuration(candidate.NodeID, "timeout", time.Since(start))
			return
		}

		c.traffic.SetNodeState(candidate.NodeID, cluster_pkg.TrafficStateWarming, "activation_pending")
		metric.SetNodeTrafficState(candidate.NodeID, cluster_pkg.TrafficStateWarming)
		if err := c.activationProber.ProbeGRPC(ctx, candidate.GRPCEndpoint, c.baseCluster.GRPC.TLS, c.baseCluster.GRPC.AllowInsecure); err != nil {
			successes = 0
			c.markActivationProbeFailure(candidate.NodeID, "grpc_probe_failed")
			time.Sleep(c.activationRetryInterval)
			continue
		}
		if err := c.activationProber.ProbeHealth(ctx, c.healthURL(candidate)); err != nil {
			successes = 0
			c.markActivationProbeFailure(candidate.NodeID, "health_probe_failed")
			time.Sleep(c.activationRetryInterval)
			continue
		}
		if err := c.activationProber.ProbeMQTT(ctx, c.mqttAddress(candidate)); err != nil {
			successes = 0
			c.markActivationProbeFailure(candidate.NodeID, "mqtt_probe_failed")
			metric.RecordNodeMQTTReadinessFailure(candidate.NodeID, "mqtt_probe_failed")
			time.Sleep(c.activationRetryInterval)
			continue
		}

		metric.SetNodeMQTTReady(candidate.NodeID, true)
		successes++
		if successes < c.activationSuccessThreshold {
			time.Sleep(c.activationWarmup)
			continue
		}
		c.traffic.SetNodeState(candidate.NodeID, cluster_pkg.TrafficStateReady, "")
		metric.SetNodeTrafficState(candidate.NodeID, cluster_pkg.TrafficStateReady)
		metric.RecordNodeActivationDuration(candidate.NodeID, "success", time.Since(start))
		return
	}
}

func (c *K8sControlClient) markActivationProbeFailure(nodeID uint64, reason string) {
	c.traffic.SetNodeState(nodeID, cluster_pkg.TrafficStateJoining, reason)
	metric.SetNodeTrafficState(nodeID, cluster_pkg.TrafficStateJoining)
	metric.RecordNodeActivationFailure(nodeID, reason)
}

func (c *K8sControlClient) healthURL(candidate api.ConsoleCandidateNode) string {
	return "http://" + c.podDNSName(candidate.PodName) + ":9526/health"
}

func (c *K8sControlClient) mqttAddress(candidate api.ConsoleCandidateNode) string {
	return c.podDNSName(candidate.PodName) + ":1883"
}

func (c *K8sControlClient) podListURL() (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(c.cfg.APIServer), "/")
	if baseURL == "" {
		baseURL = defaultK8sAPIServer
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse k8s api server: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("k8s api server must include scheme and host")
	}
	parsed.Path = path.Join(
		parsed.Path,
		"/api/v1/namespaces/"+url.PathEscape(c.cfg.Namespace)+"/pods",
	)
	query := parsed.Query()
	query.Set("labelSelector", c.cfg.LabelSelector)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *K8sControlClient) bearerToken() (string, error) {
	tokenFile := strings.TrimSpace(c.cfg.TokenFile)
	if tokenFile == "" {
		return "", nil
	}
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read k8s service account token: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (c *K8sControlClient) runtimeNode(pod k8sPod) api.ConsoleRuntimeNode {
	podName := strings.TrimSpace(pod.Metadata.Name)
	host := c.podDNSName(podName)
	status := strings.ToLower(strings.TrimSpace(pod.Status.Phase))
	if status == "" {
		status = "unknown"
	}
	return api.ConsoleRuntimeNode{
		Name:        podName,
		ServiceName: podName,
		BrokerURL:   "mqtt://" + host + ":1883",
		HealthURL:   "http://" + host + ":9526/health",
		Status:      status,
		Health:      podHealth(pod),
		Container:   podName,
	}
}

func (p *defaultActivationProber) ProbeHealth(ctx context.Context, healthURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health returned %d", resp.StatusCode)
	}
	return nil
}

func (p *defaultActivationProber) ProbeMQTT(ctx context.Context, address string) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	return conn.Close()
}

func (p *defaultActivationProber) ProbeGRPC(ctx context.Context, endpoint string, tlsCfg config.TLS, allowInsecure bool) error {
	transportCreds, err := grpcProbeTransportCredentials(tlsCfg, allowInsecure)
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(transportCreds), grpc.WithNoProxy())
	if err != nil {
		return err
	}
	defer conn.Close()

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := grpc_health_v1.NewHealthClient(conn).Check(checkCtx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("grpc health status %s", resp.GetStatus().String())
	}
	return nil
}

func grpcProbeTransportCredentials(tlsCfg config.TLS, allowInsecure bool) (credentials.TransportCredentials, error) {
	if tlsCfg.Enabled {
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		if strings.TrimSpace(tlsCfg.CAFile) != "" {
			pemData, err := os.ReadFile(tlsCfg.CAFile)
			if err != nil {
				return nil, fmt.Errorf("read cluster.grpc.tls.ca_file failed: %w", err)
			}
			pool := x509.NewCertPool()
			if ok := pool.AppendCertsFromPEM(pemData); !ok {
				return nil, fmt.Errorf("parse cluster.grpc.tls.ca_file failed")
			}
			tlsConfig.RootCAs = pool
		}
		return credentials.NewTLS(tlsConfig), nil
	}
	if allowInsecure {
		return insecure.NewCredentials(), nil
	}
	return nil, fmt.Errorf("cluster grpc probe requires tls or allow_insecure")
}

func (c *K8sControlClient) candidateNode(pod k8sPod) api.ConsoleCandidateNode {
	podName := strings.TrimSpace(pod.Metadata.Name)
	nodeID, nodeIDReason := c.nodeID(pod)
	raftAddress := c.annotationOrDefault(
		pod,
		c.cfg.RaftAddressAnnotation,
		c.podDNSName(podName)+":"+strconv.Itoa(c.cfg.RaftPort),
	)
	grpcEndpoint := c.annotationOrDefault(
		pod,
		c.cfg.GRPCEndpointAnnotation,
		c.podDNSName(podName)+":"+strconv.Itoa(c.cfg.GRPCPort),
	)
	node := api.ConsoleCandidateNode{
		NodeID:       nodeID,
		Name:         podName,
		Namespace:    podNamespace(pod, c.cfg.Namespace),
		PodName:      podName,
		PodIP:        pod.Status.PodIP,
		ServiceName:  podName,
		RaftAddress:  raftAddress,
		GRPCEndpoint: grpcEndpoint,
		Phase:        pod.Status.Phase,
	}
	node.JoinEligible, node.Reason = candidateEligibility(pod, nodeID, raftAddress, grpcEndpoint, nodeIDReason)
	return node
}

func (c *K8sControlClient) nodeMeta(candidate api.ConsoleCandidateNode) *cluster_pkg.NodeMeta {
	clusterCfg := c.baseCluster
	clusterCfg.LocalNodeID = candidate.NodeID
	clusterCfg.LocalNodeAddress = candidate.RaftAddress
	clusterCfg.Join = true
	clusterCfg.GRPC.Endpoint = candidate.GRPCEndpoint
	return &cluster_pkg.NodeMeta{Cluster: clusterCfg}
}

func (c *K8sControlClient) nodeID(pod k8sPod) (uint64, string) {
	if value := annotationValue(pod, c.cfg.NodeIDAnnotation); value != "" {
		nodeID, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return 0, "node id annotation is invalid"
		}
		return nodeID, ""
	}
	ordinal, ok := statefulSetOrdinal(pod.Metadata.Name)
	if !ok {
		return 0, "node id is missing"
	}
	return uint64(ordinal + 1), ""
}

func (c *K8sControlClient) annotationOrDefault(pod k8sPod, key string, fallback string) string {
	if value := annotationValue(pod, key); value != "" {
		return value
	}
	return fallback
}

func (c *K8sControlClient) podDNSName(podName string) string {
	return fmt.Sprintf(
		"%s.%s.%s.svc.cluster.local",
		podName,
		strings.TrimSpace(c.cfg.ServiceName),
		strings.TrimSpace(c.cfg.Namespace),
	)
}

func candidateEligibility(
	pod k8sPod,
	nodeID uint64,
	raftAddress string,
	grpcEndpoint string,
	nodeIDReason string,
) (bool, string) {
	if strings.TrimSpace(nodeIDReason) != "" {
		return false, nodeIDReason
	}
	if nodeID == 0 {
		return false, "node id is required"
	}
	if !strings.EqualFold(pod.Status.Phase, "Running") {
		return false, "pod is not running"
	}
	if !podReady(pod) {
		return false, "pod is not ready"
	}
	if strings.TrimSpace(raftAddress) == "" {
		return false, "raft address is required"
	}
	if strings.TrimSpace(grpcEndpoint) == "" {
		return false, "grpc endpoint is required"
	}
	return true, ""
}

func membershipActionResult(
	candidate api.ConsoleCandidateNode,
	result *raft_pkg.MembershipResult,
	err error,
) *api.ConsoleClusterNodeActionResult {
	status := "joined"
	message := "node joined raft cluster"
	if err != nil {
		status = "failed"
		message = "node join failed"
	}
	out := &api.ConsoleClusterNodeActionResult{
		Node:    candidate.PodName,
		Action:  "join",
		Status:  status,
		Message: message,
	}
	if result == nil {
		return out
	}
	out.RaftGroups = make([]api.ConsoleRaftGroupJoinResult, 0, len(result.Groups))
	for _, group := range result.Groups {
		out.RaftGroups = append(out.RaftGroups, api.ConsoleRaftGroupJoinResult{
			ClusterID:   group.ClusterID,
			ClusterName: group.ClusterName,
			Status:      group.Status,
			Error:       group.Error,
		})
	}
	return out
}

func candidateMatches(candidate api.ConsoleCandidateNode, node string) bool {
	if candidate.Name == node || candidate.PodName == node || candidate.ServiceName == node {
		return true
	}
	return strconv.FormatUint(candidate.NodeID, 10) == node
}

func podNamespace(pod k8sPod, fallback string) string {
	if strings.TrimSpace(pod.Metadata.Namespace) != "" {
		return strings.TrimSpace(pod.Metadata.Namespace)
	}
	return strings.TrimSpace(fallback)
}

func podHealth(pod k8sPod) string {
	if podReady(pod) {
		return "healthy"
	}
	return "unready"
}

func podReady(pod k8sPod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

func annotationValue(pod k8sPod, key string) string {
	key = strings.TrimSpace(key)
	if key == "" || pod.Metadata.Annotations == nil {
		return ""
	}
	return strings.TrimSpace(pod.Metadata.Annotations[key])
}

func statefulSetOrdinal(name string) (int, bool) {
	index := strings.LastIndex(name, "-")
	if index < 0 || index == len(name)-1 {
		return 0, false
	}
	ordinal, err := strconv.Atoi(name[index+1:])
	if err != nil || ordinal < 0 {
		return 0, false
	}
	return ordinal, true
}

func k8sConfigWithDefaults(cfg config.ConsoleK8sControl) config.ConsoleK8sControl {
	if strings.TrimSpace(cfg.APIServer) == "" {
		cfg.APIServer = defaultK8sAPIServer
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		cfg.ServiceName = "skytree-headless"
	}
	if strings.TrimSpace(cfg.NodeIDAnnotation) == "" {
		cfg.NodeIDAnnotation = "skytree.io/node-id"
	}
	if strings.TrimSpace(cfg.RaftAddressAnnotation) == "" {
		cfg.RaftAddressAnnotation = "skytree.io/raft-address"
	}
	if strings.TrimSpace(cfg.GRPCEndpointAnnotation) == "" {
		cfg.GRPCEndpointAnnotation = "skytree.io/grpc-endpoint"
	}
	if strings.TrimSpace(cfg.TokenFile) == "" {
		cfg.TokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	}
	if strings.TrimSpace(cfg.CAFile) == "" {
		cfg.CAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	}
	if cfg.RaftPort <= 0 {
		cfg.RaftPort = 8080
	}
	if cfg.GRPCPort <= 0 {
		cfg.GRPCPort = 8091
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 3 * time.Second
	}
	if cfg.JoinTimeout <= 0 {
		cfg.JoinTimeout = 10 * time.Second
	}
	return cfg
}

func newK8sHTTPClient(cfg config.ConsoleK8sControl) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if pool := k8sCertPool(cfg.CAFile); pool != nil {
		transport.TLSClientConfig = &tls.Config{
			RootCAs:    pool,
			MinVersion: tls.VersionTLS12,
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   cfg.RequestTimeout,
	}
}

func k8sCertPool(caFile string) *x509.CertPool {
	caFile = strings.TrimSpace(caFile)
	if caFile == "" {
		return nil
	}
	data, err := os.ReadFile(caFile)
	if err != nil {
		return nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil
	}
	return pool
}
