package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// GetShareGroupMembers 返回指定共享订阅组内的成员客户端和实际 topic filter。
func (t *MemorySubCenter) GetShareGroupMembers(ctx context.Context, req *proto2.GetShareGroupMembersRequest) (*proto2.GetShareGroupMembersResponse, error) {
	_ = ctx
	shareGroup := req.GetShareGroup()
	if shareGroup == "" {
		return &proto2.GetShareGroupMembersResponse{Members: []*proto2.ShareGroupMember{}}, nil
	}

	// 使用 clientID + topicFilter 去重，避免同一成员被多个分片重复返回。
	memberMap := make(map[string]*proto2.ShareGroupMember)

	t.shardMux.RLock()
	defer t.shardMux.RUnlock()

	// 共享订阅按 "$share/<group>/..." 存在各分片中，需要遍历所有分片。
	for _, shard := range t.shards {
		shard.mux.RLock()
		// 遍历当前分片内的所有客户端订阅。
		for clientID, subTopics := range shard.sub.clients.Clients {
			if subTopics == nil {
				continue
			}
			// 检查该客户端的每个 topic filter。
			for topicFilter, subOption := range subTopics.Topics {
				if subOption == nil {
					continue
				}
				// 只处理共享订阅格式。
				if !isSharedSubscription(topicFilter) {
					continue
				}
				// 从共享订阅过滤器中解析 share group 和真实 topic filter。
				parsedShareGroup, actualTopicFilter, err := parseSharedSubscription(topicFilter)
				if err != nil {
					continue
				}
				// 只保留目标 share group 的成员。
				if parsedShareGroup != shareGroup {
					continue
				}
				// 写入去重 map，key 使用 clientID 和真实 topic filter。
				key := clientID + "|" + actualTopicFilter
				if _, exists := memberMap[key]; !exists {
					memberMap[key] = &proto2.ShareGroupMember{
						ClientID:    clientID,
						TopicFilter: actualTopicFilter,
					}
				}
			}
		}
		shard.mux.RUnlock()
	}

	// 转换成稳定有序的 slice，保证测试和上层消费可预测。
	members := make([]*proto2.ShareGroupMember, 0, len(memberMap))
	for _, member := range memberMap {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].GetClientID() != members[j].GetClientID() {
			return members[i].GetClientID() < members[j].GetClientID()
		}
		return members[i].GetTopicFilter() < members[j].GetTopicFilter()
	})

	return &proto2.GetShareGroupMembersResponse{Members: members}, nil
}

// isSharedSubscription 判断 topic filter 是否为共享订阅格式。
func isSharedSubscription(topicFilter string) bool {
	return len(topicFilter) > 7 && topicFilter[:7] == "$share/"
}

// parseSharedSubscription 解析 "$share/<group>/<topicFilter>" 格式并返回共享组和真实过滤器。
func parseSharedSubscription(topicFilter string) (shareGroup string, actualTopicFilter string, err error) {
	if !isSharedSubscription(topicFilter) {
		return "", "", fmt.Errorf("not a shared subscription: %s", topicFilter)
	}

	// 移除 "$share/" 前缀。
	withoutPrefix := topicFilter[7:]
	if withoutPrefix == "" {
		return "", "", fmt.Errorf("invalid shared subscription format: missing share name")
	}

	// 第一个层级是共享组，剩余部分还原为真实 topic filter。
	parts := strings.Split(withoutPrefix, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid shared subscription format: missing topic filter")
	}

	shareGroup = parts[0]
	if shareGroup == "" {
		return "", "", fmt.Errorf("invalid shared subscription format: empty share name")
	}
	if strings.ContainsAny(shareGroup, "+#") {
		return "", "", fmt.Errorf("invalid shared subscription format: share name contains wildcard")
	}

	// 将剩余层级重新拼回真实 topic filter。
	actualTopicFilter = strings.Join(parts[1:], "/")
	if actualTopicFilter == "" {
		return "", "", fmt.Errorf("invalid shared subscription format: empty topic filter")
	}

	return shareGroup, actualTopicFilter, nil
}
