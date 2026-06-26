package memory

import (
	"context"

	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// GetSubTree 返回指定 topic 下的订阅树快照；根查询会合并所有分片。
func (t *MemorySubCenter) GetSubTree(ctx context.Context, req *proto2.GetSubTreeRequest) (*proto2.GetSubTreeResponse, error) {
	_ = ctx
	topic := req.GetTopic()
	if topic == "" {
		topic = "/"
	}
	maxDepth := int(req.GetMaxDepth())
	if maxDepth <= 0 {
		maxDepth = 8
	}

	// 根节点查询需要把所有分片合并成一个虚拟根。
	if topic == "/" {
		virtualRoot := &proto2.TreeNode{
			TopicSection: "/",
			Topic:        "/",
			Client:       make(map[string]*proto2.SubOption),
			ChildNode:    make(map[string]*proto2.TreeNode),
			ClusterSub:   make(map[string]int64),
		}

		t.shardMux.RLock()
		for _, shard := range t.shards {
			shard.mux.RLock()
			src := cloneNodeLimited(shard.sub.root, maxDepth)
			shard.mux.RUnlock()
			mergeTreeNode(virtualRoot, src)
		}
		t.shardMux.RUnlock()

		return &proto2.GetSubTreeResponse{Root: virtualRoot}, nil
	}

	// 非根查询先定位分片，再在该分片内查找节点。
	shardKey := shardKeyFromTopic(topic)
	shard, ok := t.getShard(shardKey)
	if !ok {
		// 未找到分片时返回空节点，保持 API 响应结构稳定。
		return &proto2.GetSubTreeResponse{Root: emptyTreeNode(topic)}, nil
	}

	sections := topicutil.SplitTopicLevels(topic)
	shard.mux.RLock()
	node := shard.sub.root
	for _, sec := range sections {
		if node == nil || node.ChildNode == nil {
			node = nil
			break
		}
		child, exists := node.ChildNode[sec]
		if !exists {
			node = nil
			break
		}
		node = child
	}
	var out *proto2.TreeNode
	if node != nil {
		out = cloneNodeLimited(node, maxDepth)
	}
	shard.mux.RUnlock()

	if out == nil {
		out = emptyTreeNode(topic)
	}
	return &proto2.GetSubTreeResponse{Root: out}, nil
}

// emptyTreeNode 构造一个没有客户端和子节点的稳定空树节点。
func emptyTreeNode(topic string) *proto2.TreeNode {
	return &proto2.TreeNode{
		TopicSection: lastTopicSection(topic),
		Topic:        topic,
		Client:       make(map[string]*proto2.SubOption),
		ChildNode:    make(map[string]*proto2.TreeNode),
		ClusterSub:   make(map[string]int64),
	}
}

// cloneNodeLimited 按最大深度复制订阅树节点，避免调试接口一次返回过大的树。
func cloneNodeLimited(n *proto2.TreeNode, maxDepth int) *proto2.TreeNode {
	if n == nil {
		return nil
	}
	out := &proto2.TreeNode{
		TopicSection: n.GetTopicSection(),
		Topic:        n.GetTopic(),
		Client:       make(map[string]*proto2.SubOption, len(n.GetClient())),
		ChildNode:    make(map[string]*proto2.TreeNode),
		ClusterSub:   make(map[string]int64, len(n.GetClusterSub())),
	}
	for k, v := range n.GetClient() {
		if v == nil {
			continue
		}
		out.Client[k] = &proto2.SubOption{
			QoS:               v.GetQoS(),
			RetainAsPublished: v.GetRetainAsPublished(),
			Topic:             v.GetTopic(),
		}
	}
	for k, v := range n.GetClusterSub() {
		out.ClusterSub[k] = v
	}
	if maxDepth <= 0 {
		return out
	}
	for k, child := range n.GetChildNode() {
		out.ChildNode[k] = cloneNodeLimited(child, maxDepth-1)
	}
	return out
}

// mergeTreeNode 将源树节点合并到目标树节点，用于根查询合并所有分片。
func mergeTreeNode(dst, src *proto2.TreeNode) {
	if dst == nil || src == nil {
		return
	}
	mergeTreeMetadata(dst, src)
	mergeTreeClients(dst, src)
	mergeTreeClusterSubs(dst, src)
	mergeTreeChildren(dst, src)
}

// mergeTreeMetadata 合并节点基本元数据，目标已有值时保持不变。
func mergeTreeMetadata(dst, src *proto2.TreeNode) {
	if dst.TopicSection == "" && src.GetTopicSection() != "" {
		dst.TopicSection = src.GetTopicSection()
	}
	if dst.Topic == "" && src.GetTopic() != "" {
		dst.Topic = src.GetTopic()
	}
}

// mergeTreeClients 合并节点客户端订阅，同一客户端保留 QoS 更高的订阅。
func mergeTreeClients(dst, src *proto2.TreeNode) {
	if dst.Client == nil {
		dst.Client = make(map[string]*proto2.SubOption, len(src.GetClient()))
	}
	for clientID, opt := range src.GetClient() {
		if opt == nil {
			continue
		}
		if old, ok := dst.Client[clientID]; !ok || opt.GetQoS() > old.GetQoS() {
			dst.Client[clientID] = cloneMergedSubOption(opt)
		}
	}
}

// cloneMergedSubOption 复制合并树需要暴露的订阅字段。
func cloneMergedSubOption(opt *proto2.SubOption) *proto2.SubOption {
	return &proto2.SubOption{
		QoS:               opt.GetQoS(),
		RetainAsPublished: opt.GetRetainAsPublished(),
		Topic:             opt.GetTopic(),
	}
}

// mergeTreeClusterSubs 合并集群订阅计数，同一 key 保留较大值。
func mergeTreeClusterSubs(dst, src *proto2.TreeNode) {
	if dst.ClusterSub == nil {
		dst.ClusterSub = make(map[string]int64, len(src.GetClusterSub()))
	}
	for k, v := range src.GetClusterSub() {
		if old, ok := dst.ClusterSub[k]; !ok || v > old {
			dst.ClusterSub[k] = v
		}
	}
}

// mergeTreeChildren 递归合并子节点。
func mergeTreeChildren(dst, src *proto2.TreeNode) {
	if dst.ChildNode == nil {
		dst.ChildNode = make(map[string]*proto2.TreeNode, len(src.GetChildNode()))
	}
	for k, child := range src.GetChildNode() {
		if child == nil {
			continue
		}
		if existing, ok := dst.ChildNode[k]; ok && existing != nil {
			mergeTreeNode(existing, child)
		} else {
			dst.ChildNode[k] = child
		}
	}
}

// lastTopicSection 返回 topic 的最后一个层级，用于空节点展示。
func lastTopicSection(topic string) string {
	levels := topicutil.SplitTopicLevels(topic)
	if len(levels) == 0 {
		return "/"
	}
	last := levels[len(levels)-1]
	if last == "" {
		return "/"
	}
	return last
}
