package memory

import (
	"fmt"
	"strings"

	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// ------------------------------------------------------- Normal SubTopic -------------------------------------------------------//

// SubCore 保存单个分片内的订阅 Trie 和 client 到订阅列表的索引。
type SubCore struct {
	core *proto.SubCore

	root    *proto.TreeNode
	clients *proto.ClientManager
}

// NewSubCore 创建一个空的订阅 Trie。
func NewSubCore() *SubCore {
	s := &SubCore{
		core: &proto.SubCore{
			Root: &proto.TopicSubTree{
				TreeRoot: &proto.TreeNode{
					TopicSection: "/",
					Topic:        "/",
					Client:       make(map[string]*proto.SubOption, 1000),
					ChildNode:    make(map[string]*proto.TreeNode, 10000),
				},
			},
			Clients: &proto.ClientManager{
				Clients: make(map[string]*proto.ClientSubTopic, 10000),
			},
		},
	}

	s.root = s.core.Root.TreeRoot
	s.clients = s.core.Clients

	return s
}

// NewSubCoreFromProto 从快照中的 proto 状态恢复订阅 Trie，并补齐缺失字段。
func NewSubCoreFromProto(core *proto.SubCore) *SubCore {
	if core == nil {
		return NewSubCore()
	}
	if core.Root == nil {
		core.Root = &proto.TopicSubTree{}
	}
	if core.Root.TreeRoot == nil {
		core.Root.TreeRoot = &proto.TreeNode{
			TopicSection: "/",
			Topic:        "/",
			Client:       make(map[string]*proto.SubOption, 1000),
			ChildNode:    make(map[string]*proto.TreeNode, 10000),
		}
	}
	if core.Clients == nil {
		core.Clients = &proto.ClientManager{
			Clients: make(map[string]*proto.ClientSubTopic, 10000),
		}
	}
	if core.Clients.Clients == nil {
		core.Clients.Clients = make(map[string]*proto.ClientSubTopic, 10000)
	}
	s := &SubCore{core: core}
	s.root = core.Root.TreeRoot
	s.clients = core.Clients
	return s
}

// validateTopicFilter 校验订阅过滤器是否符合 MQTT 通配符规则。
func validateTopicFilter(filter string) error {
	// '#' 必须独占一个层级且只能位于最后一层；'+' 必须独占一个层级。
	if filter == "" {
		return fmt.Errorf("invalid topic filter: empty")
	}
	levels := topicutil.SplitTopicLevels(filter)
	for i, lv := range levels {
		if lv == "#" {
			if i != len(levels)-1 {
				return fmt.Errorf("invalid topic filter: # must be last level")
			}
			continue
		}
		if strings.Contains(lv, "#") {
			return fmt.Errorf("invalid topic filter: # must occupy entire level")
		}
		if lv != "+" && strings.Contains(lv, "+") {
			return fmt.Errorf("invalid topic filter: + must occupy entire level")
		}
	}
	return nil
}

// validatePublishTopic 校验发布 topic，发布路径不能包含 MQTT 通配符。
func validatePublishTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("invalid publish topic: empty")
	}
	levels := topicutil.SplitTopicLevels(topic)
	for _, lv := range levels {
		if lv == "+" || lv == "#" || strings.Contains(lv, "+") || strings.Contains(lv, "#") {
			return fmt.Errorf("invalid publish topic: contains wildcard")
		}
	}
	return nil
}

// isMoreSpecificFilter 判断两个过滤器的优先级，用于重叠订阅合并时选择更具体的规则。
func isMoreSpecificFilter(a, b string) bool {
	// 优先选择通配符更少的过滤器；相同时选择层级更深的；仍相同时按字典序稳定排序。
	aw, bw := 0, 0
	al, bl := topicutil.SplitTopicLevels(a), topicutil.SplitTopicLevels(b)
	for _, lv := range al {
		if lv == "+" || lv == "#" {
			aw++
		}
	}
	for _, lv := range bl {
		if lv == "+" || lv == "#" {
			bw++
		}
	}
	if aw != bw {
		return aw < bw
	}
	if len(al) != len(bl) {
		return len(al) > len(bl)
	}
	return a < b
}

// allowRootWildcardsForPublish 判断根级通配符是否可以匹配该发布 topic。
func allowRootWildcardsForPublish(topicLevels []string) bool {
	if len(topicLevels) == 0 {
		return true
	}
	return !strings.HasPrefix(topicLevels[0], "$")
}

// createSub 将一个客户端订阅写入分片内的 Trie 和客户端反向索引。
func (t *SubCore) createSub(clientID string, subOption *proto.SubOption) error {
	var (
		topicFilter = subOption.GetTopic()
	)
	if err := validateTopicFilter(topicFilter); err != nil {
		return err
	}
	topicLevels := topicutil.SplitTopicLevels(topicFilter)
	if len(topicLevels) == 0 {
		return fmt.Errorf("invalid topic filter: empty")
	}

	currentNode := t.root

	for _, level := range topicLevels {
		if _, exists := currentNode.ChildNode[level]; !exists {
			if currentNode.ChildNode == nil {
				currentNode.ChildNode = make(map[string]*proto.TreeNode, 10000)
			}
			// 当前层级不存在时创建新的 Trie 节点。
			currentNode.ChildNode[level] = &proto.TreeNode{
				TopicSection: level,
				Client:       make(map[string]*proto.SubOption),
				ChildNode:    make(map[string]*proto.TreeNode),
			}
		}
		// 移动到当前层级对应的子节点。
		currentNode = currentNode.ChildNode[level]
	}

	currentNode.Topic = topicFilter

	// 所有层级处理完成后，在终止节点上保存客户端订阅参数。
	if currentNode.Client == nil {
		currentNode.Client = make(map[string]*proto.SubOption, 10000)
	}
	currentNode.Client[clientID] = subOption
	subTopics, ok := t.clients.Clients[clientID]
	if !ok {
		t.clients.Clients[clientID] = &proto.ClientSubTopic{
			Topics: make(map[string]*proto.SubOption, 10000),
		}
		subTopics = t.clients.Clients[clientID]
	}

	subTopics.Topics[subOption.Topic] = subOption
	return nil

}

// deleteSub 删除客户端在指定 topic filter 上的订阅。
func (t *SubCore) deleteSub(subOption *proto.SubOption, clientID string) {
	var (
		topicLevels = topicutil.SplitTopicLevels(subOption.GetTopic())
		currentNode = t.root
	)

	for _, level := range topicLevels {
		// 路径中任意层级不存在，说明该订阅不存在。
		if currentNode.ChildNode == nil {
			return
		}

		// 路径中任意层级不存在，说明该订阅不存在。
		if _, exists := currentNode.ChildNode[level]; !exists {
			return
		}

		currentNode = currentNode.ChildNode[level]
	}

	delete(currentNode.Client, clientID)

	if len(currentNode.Client) == 0 {
		currentNode.Client = nil
	}

	subTopics, ok := t.clients.Clients[clientID]
	if ok {
		delete(subTopics.Topics, subOption.Topic)
		if len(subTopics.Topics) == 0 {
			delete(t.clients.Clients, clientID)
		}
	}
}

// deleteTopic 删除指定 topic filter 下的所有客户端订阅，并返回受影响的 clientID。
func (t *SubCore) deleteTopic(topic string) (removedClientIDs []string) {
	var (
		topicLevels = topicutil.SplitTopicLevels(topic)
		currentNode = t.root
	)

	for _, level := range topicLevels {
		// 路径中任意层级不存在，说明该 topic filter 没有订阅。
		if currentNode.ChildNode == nil {
			return
		}

		// 路径中任意层级不存在，说明该 topic filter 没有订阅。
		if _, exists := currentNode.ChildNode[level]; !exists {
			return
		}

		currentNode = currentNode.ChildNode[level]
	}

	for clientID := range currentNode.Client {
		if subTopics, ok := t.clients.Clients[clientID]; ok {
			delete(subTopics.Topics, topic)
			if len(subTopics.Topics) == 0 {
				delete(t.clients.Clients, clientID)
			}
		}
		removedClientIDs = append(removedClientIDs, clientID)
	}
	currentNode.Client = nil
	return removedClientIDs
}

// deleteClient 删除客户端在当前分片内的所有订阅。
func (t *SubCore) deleteClient(clientID string) {
	subTopic, ok := t.clients.Clients[clientID]
	if !ok {
		return
	}
	for _, subOption := range subTopic.Topics {
		t.deleteSub(subOption, clientID)
	}
}

// matchTopic 返回匹配发布 topic 的订阅过滤器及其最大 QoS。
func (t *SubCore) matchTopic(topic string) map[string]int32 {
	result := make(map[string]int32)
	t.walkMatchingTopicNodes(topic, func(node *proto.TreeNode) {
		if node == nil || node.Topic == "" {
			return
		}
		if qos := nodeMaxQoS(node); qos > result[node.Topic] {
			result[node.Topic] = qos
		}
	})
	return result
}

// matchTopicClient 返回匹配发布 topic 的客户端订阅，并按客户端折叠为最优订阅参数。
func (t *SubCore) matchTopicClient(topic string) map[string]*proto.SubOption {
	result := make(map[string]*proto.SubOption)
	t.walkMatchingTopicNodes(topic, func(node *proto.TreeNode) {
		mergeMatchedNodeClients(result, node)
	})
	return result
}

// matchTopicClientV2 返回每个客户端命中的全部订阅，不在 sub 层折叠重叠订阅。
// 客户端中心化投递需要保留完整匹配结果，由 client 层负责去重和最终投递参数选择。
func (t *SubCore) matchTopicClientV2(topic string) map[string][]*proto.MatchedSubscription {
	result := make(map[string][]*proto.MatchedSubscription)
	t.walkMatchingTopicNodes(topic, func(node *proto.TreeNode) {
		appendMatchedNodeClients(result, node)
	})
	return result
}

// walkMatchingTopicNodes 按 MQTT 通配符规则遍历所有可匹配发布 topic 的 Trie 节点。
func (t *SubCore) walkMatchingTopicNodes(topic string, visit func(node *proto.TreeNode)) {
	topicLevels := topicutil.SplitTopicLevels(topic)
	if err := validatePublishTopic(topic); err != nil || len(topicLevels) == 0 {
		return
	}
	rootWildcardsAllowed := allowRootWildcardsForPublish(topicLevels)
	var match func(node *proto.TreeNode, depth int)
	match = func(node *proto.TreeNode, depth int) {
		if node == nil {
			return
		}
		if depth == 0 && node == t.root {
			t.walkSharedSubscriptionRoots(node, match)
		}
		if hashChild := matchingHashChild(node, depth, rootWildcardsAllowed); hashChild != nil {
			visit(hashChild)
		}
		if depth == len(topicLevels) {
			visit(node)
			return
		}
		if node.ChildNode == nil {
			return
		}
		level := topicLevels[depth]
		if child, ok := node.ChildNode[level]; ok {
			match(child, depth+1)
		}
		if plusChild, ok := node.ChildNode["+"]; ok && (depth != 0 || rootWildcardsAllowed) {
			match(plusChild, depth+1)
		}
	}
	match(t.root, 0)
}

// walkSharedSubscriptionRoots 将 $share/<group>/... 的实际过滤器纳入普通匹配流程。
func (t *SubCore) walkSharedSubscriptionRoots(root *proto.TreeNode, match func(node *proto.TreeNode, depth int)) {
	if root.ChildNode == nil {
		return
	}
	shareRoot, ok := root.ChildNode["$share"]
	if !ok || shareRoot == nil || shareRoot.ChildNode == nil {
		return
	}
	for _, groupNode := range shareRoot.ChildNode {
		if groupNode != nil {
			match(groupNode, 0)
		}
	}
}

// matchingHashChild 返回当前节点下可以用 '#' 匹配剩余层级的子节点。
func matchingHashChild(node *proto.TreeNode, depth int, rootWildcardsAllowed bool) *proto.TreeNode {
	if node.ChildNode == nil || (depth == 0 && !rootWildcardsAllowed) {
		return nil
	}
	hashChild, ok := node.ChildNode["#"]
	if !ok {
		return nil
	}
	return hashChild
}

// mergeMatchedNodeClients 将节点上的客户端订阅合并到结果中，同客户端只保留最优订阅。
func mergeMatchedNodeClients(result map[string]*proto.SubOption, node *proto.TreeNode) {
	if node == nil {
		return
	}
	for clientID, incoming := range node.Client {
		if incoming == nil {
			continue
		}
		existing, ok := result[clientID]
		if !ok || incoming.GetQoS() > existing.GetQoS() {
			result[clientID] = incoming
			continue
		}
		if existing != nil && incoming.GetQoS() == existing.GetQoS() && isMoreSpecificFilter(incoming.GetTopic(), existing.GetTopic()) {
			result[clientID] = incoming
		}
	}
}

// appendMatchedNodeClients 将节点上的客户端订阅追加到多匹配结果中，不做折叠。
func appendMatchedNodeClients(result map[string][]*proto.MatchedSubscription, node *proto.TreeNode) {
	if node == nil {
		return
	}
	for clientID, opt := range node.Client {
		if opt != nil {
			result[clientID] = append(result[clientID], matchedSubscriptionFromOption(opt))
		}
	}
}

// matchedSubscriptionFromOption 将订阅参数转换成匹配结果，避免直接复制 protobuf 内部状态。
func matchedSubscriptionFromOption(opt *proto.SubOption) *proto.MatchedSubscription {
	return &proto.MatchedSubscription{
		TopicFilter:            opt.GetTopic(),
		QoS:                    opt.GetQoS(),
		RetainAsPublished:      opt.GetRetainAsPublished(),
		NoLocal:                opt.GetNoLocal(),
		RetainHandling:         opt.GetRetainHandling(),
		SubscriptionIdentifier: opt.GetSubscriptionIdentifier(),
	}
}

// matchTopicForWildcard 返回能被指定通配过滤器覆盖的已订阅 topic filter。
func (t *SubCore) matchTopicForWildcard(wildcardTopic string) []string {
	var (
		result []string

		match func(node *proto.TreeNode, levels []string, depth int)
	)

	match = func(node *proto.TreeNode, levels []string, depth int) {
		if node == nil {
			return
		}
		if depth == len(levels) {
			if node.Topic != "" {
				result = append(result, node.Topic)
			}
			return
		}

		if node.ChildNode == nil {
			return
		}

		section := levels[depth]
		if section == "+" {
			for k, child := range node.ChildNode {
				if k != "#" && k != "+" {
					match(child, levels, depth+1)
				}
			}
			return
		}

		if section == "#" {
			t.collectAllTopics(node, &result)
			return
		}

		if child, ok := node.ChildNode[section]; ok {
			match(child, levels, depth+1)
		}

	}

	levels := topicutil.SplitTopicLevels(wildcardTopic)
	match(t.root, levels, 0)
	return result
}

// nodeMaxQoS 返回节点上所有客户端订阅的最大 QoS。
func nodeMaxQoS(node *proto.TreeNode) int32 {
	var maxQoS int32
	for _, c := range node.Client {
		if c.QoS > maxQoS {
			maxQoS = c.QoS
		}
	}

	return maxQoS
}

// collectAllTopics 递归收集节点下所有具体订阅 topic filter。
func (t *SubCore) collectAllTopics(node *proto.TreeNode, topics *[]string) {
	if node.Topic != "" {
		if node.TopicSection != "#" && node.TopicSection != "+" {
			*topics = append(*topics, node.Topic)
		}
	}
	for _, child := range node.ChildNode {
		t.collectAllTopics(child, topics)
	}
}

// getClientSubscriptions 返回客户端在当前分片内的订阅副本。
func (t *SubCore) getClientSubscriptions(clientID string) map[string]*proto.SubOption {
	result := make(map[string]*proto.SubOption)
	if subTopics, ok := t.clients.Clients[clientID]; ok && subTopics != nil {
		for topic, subOption := range subTopics.Topics {
			if subOption != nil {
				result[topic] = &proto.SubOption{
					Topic:                  subOption.GetTopic(),
					QoS:                    subOption.GetQoS(),
					RetainAsPublished:      subOption.GetRetainAsPublished(),
					NoLocal:                subOption.GetNoLocal(),
					RetainHandling:         subOption.GetRetainHandling(),
					SubscriptionIdentifier: subOption.GetSubscriptionIdentifier(),
				}
			}
		}
	}
	return result
}
