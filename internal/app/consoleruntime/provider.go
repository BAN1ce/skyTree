package consoleruntime

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/api"
	"github.com/BAN1ce/skyTree/internal/app/statecenter"
	"github.com/BAN1ce/skyTree/internal/app/storeruntime"
	"github.com/BAN1ce/skyTree/config"
	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/internal/broker/retain"
	willdelay "github.com/BAN1ce/skyTree/internal/broker/willdelay"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	cluster_pkg "github.com/BAN1ce/skyTree/pkg/cluster"
	proto_session "github.com/BAN1ce/skyTree/proto/proto_session"
	proto_topic "github.com/BAN1ce/skyTree/proto/proto_topic"
)

const (
	defaultDeliveryTaskLimit = 20
	payloadPreviewBytes      = 128
)

type Dependencies struct {
	Config          config.AppConfig
	ClientManager   *brokerclient.Manager
	StateCenters    *statecenter.Runtime
	Stores          *storeruntime.Runtime
	ClusterOverview api.ClusterOverviewProvider
	ClusterState    cluster_pkg.State
	Control         ControlClient
}

type ControlClient interface {
	ListClusterNodes(ctx context.Context) ([]api.ConsoleRuntimeNode, error)
	ListCandidateNodes(ctx context.Context) ([]api.ConsoleCandidateNode, error)
	RunClusterNodeAction(ctx context.Context, node string, action string) (*api.ConsoleClusterNodeActionResult, error)
}

type Provider struct {
	cfg             config.AppConfig
	clientManager   *brokerclient.Manager
	sessionCenter   session.Center
	subCenter       subscription.Center
	willDelayCenter willdelay.Center
	retainStore     *retain.Store
	deliveryCursor  delivery.CursorStore
	backlogStore    store.DeliveryBacklogSummaryStore
	clusterOverview api.ClusterOverviewProvider
	clusterState    cluster_pkg.State
	control         ControlClient
}

func NewProvider(deps Dependencies) api.ConsoleProvider {
	p := &Provider{
		cfg:             deps.Config,
		clientManager:   deps.ClientManager,
		clusterOverview: deps.ClusterOverview,
		clusterState:    deps.ClusterState,
		control:         deps.Control,
	}
	if deps.StateCenters != nil {
		p.sessionCenter = deps.StateCenters.Session
		p.subCenter = deps.StateCenters.Subscription
		p.willDelayCenter = deps.StateCenters.WillDelay
	}
	if deps.Stores != nil {
		if deps.Stores.KeyStore != nil {
			p.retainStore = retain.NewRetainStore(deps.Stores.KeyStore)
		}
		if summaryStore, ok := deps.Stores.ClientDeliveryStore.(store.DeliveryBacklogSummaryStore); ok {
			p.backlogStore = summaryStore
		}
		p.deliveryCursor = deps.Stores.DeliveryCursorStore
	}
	return p
}

func (p *Provider) GetSummary(ctx context.Context) (*api.ConsoleSummary, error) {
	backlog := api.ClusterDeliveryBacklogOverview{Supported: false, Reason: "delivery backlog summary is unavailable"}
	if p.backlogStore != nil {
		summary, err := p.backlogStore.DeliveryBacklogSummary(ctx)
		if err != nil {
			backlog.Reason = err.Error()
		} else {
			backlog = api.ClusterDeliveryBacklogOverview{
				Supported:     true,
				PendingTasks:  summary.PendingTasks,
				ActiveClients: summary.ActiveClients,
			}
		}
	}

	return &api.ConsoleSummary{
		Timestamp:       time.Now(),
		ServerPort:      p.cfg.Server.Port,
		ClusterEnabled:  p.cfg.Cluster.Enable,
		OnlineClients:   len(p.onlineClients()),
		StorageDriver:   p.cfg.Storage.Default,
		MetricsPath:     "/metrics",
		DeliveryBacklog: backlog,
	}, nil
}

func (p *Provider) ListClients(_ context.Context, query api.ConsoleClientQuery) (*api.ConsoleClientList, error) {
	clients := p.onlineClients()
	items := make([]api.ConsoleClientSummary, 0, len(clients))
	now := time.Now()
	for _, c := range clients {
		summary := p.clientSummary(c, now)
		if query.ClientID != "" && !strings.Contains(summary.ClientID, query.ClientID) {
			continue
		}
		items = append(items, summary)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ClientID < items[j].ClientID
	})
	total := len(items)
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return &api.ConsoleClientList{
		Total: total,
		Items: items,
	}, nil
}

func (p *Provider) GetClient(ctx context.Context, clientID string) (*api.ConsoleClientDetail, bool, error) {
	var (
		clientSummary api.ConsoleClientSummary
		online        bool
	)
	if p.clientManager != nil {
		if c, ok := p.clientManager.ReadClient(clientID); ok {
			clientSummary = p.clientSummary(c, time.Now())
			online = true
		}
	}
	if !online {
		clientSummary = api.ConsoleClientSummary{ClientID: clientID, Online: false}
	}

	sessionSummary, sessionExists, err := p.readSession(ctx, clientID)
	if err != nil {
		return nil, false, err
	}
	ownerSummary, ownerExists, err := p.readSessionOwner(ctx, clientID)
	if err != nil {
		return nil, false, err
	}
	if !online && !sessionExists && !ownerExists {
		return nil, false, nil
	}

	subscriptions, err := p.readClientSubscriptions(ctx, clientID)
	if err != nil {
		return nil, false, err
	}
	delivery, err := p.readDelivery(ctx, clientID)
	if err != nil {
		return nil, false, err
	}

	return &api.ConsoleClientDetail{
		Client:        clientSummary,
		Session:       sessionSummary,
		Owner:         ownerSummary,
		Subscriptions: subscriptions,
		Delivery:      delivery,
	}, true, nil
}

func (p *Provider) GetSubscriptionTree(
	ctx context.Context,
	query api.ConsoleSubscriptionTreeQuery,
) (*api.ConsoleSubscriptionTree, error) {
	if p.subCenter == nil {
		return &api.ConsoleSubscriptionTree{}, nil
	}
	resp, err := p.subCenter.GetSubTree(ctx, &proto_topic.GetSubTreeRequest{
		Topic:    query.Topic,
		MaxDepth: query.MaxDepth,
	})
	if err != nil {
		return nil, err
	}
	return &api.ConsoleSubscriptionTree{Root: convertTreeNode(resp.GetRoot())}, nil
}

func (p *Provider) GetShareGroupMembers(
	ctx context.Context,
	shareGroup string,
) (*api.ConsoleShareGroupMemberList, error) {
	if p.subCenter == nil || strings.TrimSpace(shareGroup) == "" {
		return &api.ConsoleShareGroupMemberList{Items: []api.ConsoleShareGroupMember{}}, nil
	}
	resp, err := p.subCenter.GetShareGroupMembers(ctx, &proto_topic.GetShareGroupMembersRequest{
		ShareGroup: shareGroup,
	})
	if err != nil {
		return nil, err
	}
	items := make([]api.ConsoleShareGroupMember, 0, len(resp.GetMembers()))
	for _, member := range resp.GetMembers() {
		if member == nil {
			continue
		}
		items = append(items, api.ConsoleShareGroupMember{
			ClientID:    member.GetClientID(),
			TopicFilter: member.GetTopicFilter(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ClientID != items[j].ClientID {
			return items[i].ClientID < items[j].ClientID
		}
		return items[i].TopicFilter < items[j].TopicFilter
	})
	return &api.ConsoleShareGroupMemberList{
		Total: len(items),
		Items: items,
	}, nil
}

func (p *Provider) ListRetainMessages(
	ctx context.Context,
	query api.ConsoleRetainQuery,
) (*api.ConsoleRetainList, error) {
	if p.retainStore == nil {
		return &api.ConsoleRetainList{Items: []api.ConsoleRetainItem{}}, nil
	}
	filter := query.TopicFilter
	if filter == "" {
		filter = "#"
	}
	messages, err := p.retainStore.GetRetainMessagesByTopicFilter(filter)
	if err != nil {
		return nil, err
	}
	items := make([]api.ConsoleRetainItem, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		items = append(items, api.ConsoleRetainItem{
			Topic:             msg.GetTopic(),
			QoS:               msg.GetQos(),
			PayloadBytes:      len(msg.GetPayload()),
			PayloadPreview:    previewPayload(msg.GetPayload()),
			CreatedAtUnixNano: msg.GetCreatedAtUnixNano(),
			ExpiredAtUnixNano: msg.GetExpiredAtUnixNano(),
			PublisherClientID: msg.GetPublisherClientID(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Topic < items[j].Topic
	})
	total := len(items)
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return &api.ConsoleRetainList{Total: total, Items: items}, nil
}

func (p *Provider) ListDueWillDelayTasks(
	ctx context.Context,
	query api.ConsoleWillDelayQuery,
) (*api.ConsoleWillDelayList, error) {
	if p.willDelayCenter == nil {
		return &api.ConsoleWillDelayList{Items: []api.ConsoleWillDelayTask{}}, nil
	}
	tasks, err := p.willDelayCenter.GetDueTasks(ctx, query.BeforeUnixMicro)
	if err != nil {
		return nil, err
	}
	items := make([]api.ConsoleWillDelayTask, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		items = append(items, api.ConsoleWillDelayTask{
			ClientID:             task.GetClientID(),
			ScheduledAtUnixMicro: task.GetScheduledPublishTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ScheduledAtUnixMicro == items[j].ScheduledAtUnixMicro {
			return items[i].ClientID < items[j].ClientID
		}
		return items[i].ScheduledAtUnixMicro < items[j].ScheduledAtUnixMicro
	})
	total := len(items)
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return &api.ConsoleWillDelayList{Total: total, Items: items}, nil
}

func (p *Provider) ListClusterNodes(ctx context.Context) (*api.ConsoleClusterNodeList, error) {
	out := &api.ConsoleClusterNodeList{
		RegisteredNodes: []api.ConsoleRegisteredNode{},
		RuntimeNodes:    []api.ConsoleRuntimeNode{},
		CandidateNodes:  []api.ConsoleCandidateNode{},
	}
	if p.clusterState != nil {
		nodes, err := p.clusterState.ListNode(ctx)
		if err != nil {
			return nil, err
		}
		out.RegisteredNodes = convertRegisteredNodes(nodes)
	}
	if p.control == nil {
		out.ControlAvailable = false
		return out, nil
	}
	runtimeNodes, err := p.control.ListClusterNodes(ctx)
	if err != nil {
		out.ControlAvailable = false
		out.ControlError = "cluster control unavailable"
		return out, nil
	}
	out.ControlAvailable = true
	out.RuntimeNodes = runtimeNodes
	candidateNodes, err := p.control.ListCandidateNodes(ctx)
	if err != nil {
		out.ControlError = "cluster candidate discovery unavailable"
		return out, nil
	}
	out.CandidateNodes = markCandidateJoinState(candidateNodes, out.RegisteredNodes)
	return out, nil
}

func (p *Provider) RunClusterNodeAction(
	ctx context.Context,
	node string,
	action api.ConsoleClusterNodeAction,
) (*api.ConsoleClusterNodeActionResult, error) {
	if p.control == nil {
		return nil, errors.New("cluster control is unavailable")
	}
	return p.control.RunClusterNodeAction(ctx, node, action.Action)
}

func (p *Provider) onlineClients() []*brokerclient.Client {
	if p.clientManager == nil {
		return []*brokerclient.Client{}
	}
	return p.clientManager.Snapshot()
}

func (p *Provider) clientSummary(c *brokerclient.Client, now time.Time) api.ConsoleClientSummary {
	if c == nil {
		return api.ConsoleClientSummary{}
	}
	remoteAddr := ""
	if conn := c.GetConn(); conn != nil {
		remoteAddr = addrString(conn.RemoteAddr())
	}
	return api.ConsoleClientSummary{
		ClientID:        c.GetID(),
		Username:        c.Username,
		RemoteAddr:      remoteAddr,
		KeepAliveSecond: int64(c.GetKeepAliveTime().Seconds()),
		IdleSecond:      int64(c.IdleDurationSince(now).Seconds()),
		Online:          true,
	}
}

func (p *Provider) readSession(
	ctx context.Context,
	clientID string,
) (*api.ConsoleSessionSummary, bool, error) {
	if p.sessionCenter == nil {
		return nil, false, nil
	}
	resp, err := p.sessionCenter.GetSession(ctx, &proto_session.ReadSessionRequest{
		ClientID:    clientID,
		NowUnixNano: time.Now().UnixNano(),
	})
	if err != nil {
		return nil, false, err
	}
	if resp == nil || !resp.GetExist() || resp.GetSession() == nil {
		return nil, false, nil
	}
	return convertSession(resp.GetSession()), true, nil
}

func (p *Provider) readSessionOwner(
	ctx context.Context,
	clientID string,
) (*api.ConsoleSessionOwnerSummary, bool, error) {
	if p.sessionCenter == nil {
		return nil, false, nil
	}
	resp, err := p.sessionCenter.GetSessionOwner(ctx, &proto_session.ReadSessionOwnerRequest{ClientID: clientID})
	if err != nil {
		return nil, false, err
	}
	if resp == nil || resp.GetOwner() == nil {
		return nil, false, nil
	}
	owner := resp.GetOwner()
	return &api.ConsoleSessionOwnerSummary{
		ClientID: owner.GetClientID(),
		NodeID:   owner.GetNodeID(),
		Online:   owner.GetOnline(),
	}, true, nil
}

func (p *Provider) readClientSubscriptions(
	ctx context.Context,
	clientID string,
) ([]api.ConsoleSubscriptionSummary, error) {
	if p.subCenter == nil {
		return []api.ConsoleSubscriptionSummary{}, nil
	}
	resp, err := p.subCenter.GetClientSubscriptions(
		ctx,
		&proto_topic.GetClientSubscriptionsRequest{ClientID: clientID},
	)
	if err != nil {
		return nil, err
	}
	items := make([]api.ConsoleSubscriptionSummary, 0, len(resp.GetTopics()))
	for topic, opt := range resp.GetTopics() {
		items = append(items, convertSubscription(topic, opt))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Topic < items[j].Topic
	})
	return items, nil
}

func (p *Provider) readDelivery(ctx context.Context, clientID string) (*api.ConsoleDeliverySummary, error) {
	if p.deliveryCursor == nil {
		return nil, nil
	}
	cursor, err := p.deliveryCursor.ReadCursor(ctx, clientID)
	if err != nil && !store.IsNotFound(err) {
		return nil, err
	}
	if cursor == nil {
		return &api.ConsoleDeliverySummary{Tasks: []api.ConsoleDeliveryTask{}}, nil
	}
	tasks, err := p.deliveryCursor.ReadTasks(
		ctx,
		clientID,
		cursor.LastTS,
		cursor.LastTaskID,
		defaultDeliveryTaskLimit,
	)
	if err != nil && !store.IsNotFound(err) {
		return nil, err
	}
	return &api.ConsoleDeliverySummary{
		Cursor: convertDeliveryCursor(cursor),
		Tasks:  convertDeliveryTasks(tasks),
	}, nil
}

func convertSession(s *proto_session.Session) *api.ConsoleSessionSummary {
	if s == nil {
		return nil
	}
	unfinished := s.GetUnfinishedMessages()
	return &api.ConsoleSessionSummary{
		ClientID:                s.GetClientID(),
		Exists:                  true,
		SessionExpirySecond:     s.GetSessionExpiryInterval(),
		ExpireAtUnixNano:        s.GetExpireAtUnixNano(),
		WillMessage:             convertWillMessage(s.GetWillMessage()),
		UnfinishedMessageCount:  len(unfinished),
		UnfinishedMessageSample: convertUnfinishedMessages(unfinished, 10),
	}
}

func convertWillMessage(w *proto_session.WillMessage) *api.ConsoleWillMessageSummary {
	if w == nil {
		return nil
	}
	return &api.ConsoleWillMessageSummary{
		Topic:             w.GetTopic(),
		QoS:               w.GetQos(),
		Retain:            w.GetRetain(),
		PayloadBytes:      len(w.GetPayload()),
		WillDelaySecond:   w.GetWillDelayInterval(),
		MessageExpirySecs: w.GetMessageExpiry(),
	}
}

func convertUnfinishedMessages(
	messages []*proto_session.UnfinishedMessage,
	limit int,
) []api.ConsoleUnfinishedMessageItem {
	items := make([]api.ConsoleUnfinishedMessageItem, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		items = append(items, api.ConsoleUnfinishedMessageItem{
			MessageID:      msg.GetMessageID(),
			PacketID:       msg.GetPacketID(),
			QoS:            msg.GetQos(),
			State:          msg.GetState().String(),
			IsOutgoing:     msg.GetIsOutgoing(),
			SubscribeTopic: msg.GetSubscribeTopic(),
		})
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items
}

func convertSubscription(topic string, opt *proto_topic.SubOption) api.ConsoleSubscriptionSummary {
	if opt == nil {
		return api.ConsoleSubscriptionSummary{Topic: topic}
	}
	if topic == "" {
		topic = opt.GetTopic()
	}
	return api.ConsoleSubscriptionSummary{
		Topic:                  topic,
		QoS:                    opt.GetQoS(),
		NoLocal:                opt.GetNoLocal(),
		RetainAsPublished:      opt.GetRetainAsPublished(),
		RetainHandling:         opt.GetRetainHandling(),
		SubscriptionIdentifier: opt.GetSubscriptionIdentifier(),
	}
}

func convertTreeNode(node *proto_topic.TreeNode) *api.ConsoleSubscriptionTreeNode {
	if node == nil {
		return nil
	}
	clients := make([]api.ConsoleSubscriptionSummary, 0, len(node.GetClient()))
	for clientID, opt := range node.GetClient() {
		item := convertSubscription(node.GetTopic(), opt)
		item.ClientID = clientID
		clients = append(clients, item)
	}
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].ClientID < clients[j].ClientID
	})

	children := make([]api.ConsoleSubscriptionTreeNode, 0, len(node.GetChildNode()))
	for _, child := range node.GetChildNode() {
		if converted := convertTreeNode(child); converted != nil {
			children = append(children, *converted)
		}
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].TopicSection < children[j].TopicSection
	})

	return &api.ConsoleSubscriptionTreeNode{
		TopicSection: node.GetTopicSection(),
		Topic:        node.GetTopic(),
		Clients:      clients,
		Children:     children,
	}
}

func convertDeliveryCursor(cursor *store.DeliveryCursor) *api.ConsoleDeliveryCursor {
	if cursor == nil {
		return nil
	}
	return &api.ConsoleDeliveryCursor{
		ClientID:   cursor.ClientID,
		Generation: cursor.Generation,
		LastTS:     cursor.LastTS,
		LastTaskID: cursor.LastTaskID.String(),
		UpdatedTS:  cursor.UpdatedTS,
	}
}

func convertDeliveryTasks(tasks []*store.DeliveryTask) []api.ConsoleDeliveryTask {
	items := make([]api.ConsoleDeliveryTask, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		items = append(items, api.ConsoleDeliveryTask{
			TaskID:          task.TaskID.String(),
			MessageID:       task.MessageID.String(),
			CreatedAt:       task.TS,
			DeliveryQoS:     task.DeliveryQoS,
			Generation:      task.Generation,
			ShareGroup:      task.ShareGroup,
			SubscriptionIDs: task.SubscriptionIDs,
		})
	}
	return items
}

func convertRegisteredNodes(nodes []*cluster_pkg.NodeMeta) []api.ConsoleRegisteredNode {
	items := make([]api.ConsoleRegisteredNode, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		items = append(items, api.ConsoleRegisteredNode{
			NodeID:           node.LocalNodeID,
			LocalNodeAddress: node.LocalNodeAddress,
			GRPCAddr:         node.GRPC.Addr,
			GRPCEndpoint:     node.GRPC.Endpoint,
			Join:             node.Join,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].NodeID < items[j].NodeID
	})
	return items
}

func markCandidateJoinState(
	candidates []api.ConsoleCandidateNode,
	registered []api.ConsoleRegisteredNode,
) []api.ConsoleCandidateNode {
	joinedNodeIDs := make(map[uint64]struct{}, len(registered))
	for _, node := range registered {
		joinedNodeIDs[node.NodeID] = struct{}{}
	}
	items := make([]api.ConsoleCandidateNode, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := joinedNodeIDs[candidate.NodeID]; ok {
			candidate.Joined = true
			candidate.JoinEligible = false
			if candidate.Reason == "" {
				candidate.Reason = "node is already registered"
			}
		}
		items = append(items, candidate)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].NodeID != items[j].NodeID {
			return items[i].NodeID < items[j].NodeID
		}
		return items[i].PodName < items[j].PodName
	})
	return items
}

func previewPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	if len(payload) > payloadPreviewBytes {
		payload = payload[:payloadPreviewBytes]
	}
	return string(payload)
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}
