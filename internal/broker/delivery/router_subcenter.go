package delivery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
)

// Router implementation backed by SubCenter GetAllMatchClientV2.

type SubCenterRouter struct {
	subCenter subscription.Center
}

var jsonMarshal = json.Marshal

func NewSubCenterRouter(sc subscription.Center) *SubCenterRouter {
	return &SubCenterRouter{subCenter: sc}
}

func (r *SubCenterRouter) Route(ctx context.Context, publish *packets.Publish, publisherClientID string) (*RouteResult, error) {
	if publish == nil {
		return &RouteResult{}, nil
	}
	if r == nil || r.subCenter == nil {
		return nil, nil
	}
	matches, err := r.getMatches(ctx, publish.Topic)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return &RouteResult{}, nil
	}

	out := &RouteResult{
		Plans:           make([]ClientPlan, 0, len(matches)),
		ShareGroupTasks: make([]ShareGroupTask, 0),
	}

	shareAggMap := make(map[string]*shareAgg)
	for _, cm := range matches {
		if cm == nil || len(cm.GetMatched()) == 0 {
			continue
		}

		normalMatches, sharedMatches := splitMatches(cm.GetMatched())
		aggregateSharedMatches(cm.GetClientID(), sharedMatches, shareAggMap, publisherClientID)

		plan, ok, err := buildClientPlan(cm, publish, normalMatches, publisherClientID)
		if err != nil {
			return nil, fmt.Errorf("build route plan failed for client %s topic %s: %w", cm.GetClientID(), publish.Topic, err)
		}
		if ok {
			out.Plans = append(out.Plans, plan)
		}
	}

	shareTasks, err := buildShareGroupTasks(shareAggMap, publish, publisherClientID)
	if err != nil {
		return nil, err
	}
	out.ShareGroupTasks = shareTasks

	return out, nil
}

// getMatches loads all client matches for the given topic from SubCenter.
func (r *SubCenterRouter) getMatches(ctx context.Context, topic string) ([]*proto_topic.ClientMatch, error) {
	resp, err := r.subCenter.GetAllMatchClientV2(ctx, &proto_topic.GetAllMatchClientV2Request{Topic: topic})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.GetMatches()) == 0 {
		return nil, nil
	}
	return resp.GetMatches(), nil
}

// shareAgg aggregates shared subscriptions by (shareGroup + actualTopicFilter).
type shareAgg struct {
	shareGroup  string
	topicFilter string // actual topic filter (without $share/<group>/ prefix)
	winner      *proto_topic.MatchedSubscription
	subIDs      []int32
}

// splitMatches separates normal subscriptions from shared subscriptions.
func splitMatches(matched []*proto_topic.MatchedSubscription) ([]*proto_topic.MatchedSubscription, []*proto_topic.MatchedSubscription) {
	normalMatches := make([]*proto_topic.MatchedSubscription, 0, len(matched))
	sharedMatches := make([]*proto_topic.MatchedSubscription, 0, 2)
	for _, ms := range matched {
		if ms == nil {
			continue
		}
		if sharedsubscription.IsSharedSubscription(ms.GetTopicFilter()) {
			sharedMatches = append(sharedMatches, ms)
			continue
		}
		normalMatches = append(normalMatches, ms)
	}
	return normalMatches, sharedMatches
}

// aggregateSharedMatches updates share-group aggregations for shared subscriptions.
func aggregateSharedMatches(
	clientID string,
	sharedMatches []*proto_topic.MatchedSubscription,
	shareAggMap map[string]*shareAgg,
	publisherClientID string,
) {
	for _, ms := range sharedMatches {
		if ms == nil {
			continue
		}
		if publisherClientID != "" && publisherClientID == clientID && ms.GetNoLocal() {
			continue
		}
		shareGroup, actualTopicFilter, err := sharedsubscription.ParseSharedSubscription(ms.GetTopicFilter())
		if err != nil || shareGroup == "" || actualTopicFilter == "" {
			continue
		}
		key := shareGroup + "|" + actualTopicFilter
		a, ok := shareAggMap[key]
		if !ok {
			a = &shareAgg{
				shareGroup:  shareGroup,
				topicFilter: actualTopicFilter,
				subIDs:      make([]int32, 0, 8),
			}
			shareAggMap[key] = a
		}
		if isBetterWinner(ms, a.winner) {
			a.winner = ms
		}
		if sid := ms.GetSubscriptionIdentifier(); sid > 0 {
			a.subIDs = append(a.subIDs, sid)
		}
	}
}

// buildClientPlan builds a client plan from normal (non-shared) subscriptions.
func buildClientPlan(cm *proto_topic.ClientMatch, publish *packets.Publish, normalMatches []*proto_topic.MatchedSubscription, publisherClientID string) (ClientPlan, bool, error) {
	if cm == nil || publish == nil || len(normalMatches) == 0 {
		return ClientPlan{}, false, nil
	}
	if cm.GetClientID() == "" {
		return ClientPlan{}, false, nil
	}

	filteredMatches := filterNoLocalMatches(normalMatches, cm.GetClientID(), publisherClientID)
	if len(filteredMatches) == 0 {
		return ClientPlan{}, false, nil
	}

	winner, subIDs := selectWinnerAndSubIDs(filteredMatches)
	if !winner.set {
		return ClientPlan{}, false, nil
	}
	b, err := jsonMarshal(subIDs)
	if err != nil {
		return ClientPlan{}, false, fmt.Errorf("marshal subscription ids failed: %w", err)
	}
	effQoS := int(winner.qos)
	if int(publish.QoS) < effQoS {
		effQoS = int(publish.QoS)
	}
	return ClientPlan{
		ClientID:            cm.GetClientID(),
		DeliveryQoS:         effQoS,
		SubscriptionIDsJSON: string(b),
		WinnerNoLocal:       winner.noLocal,
		WinnerRAP:           winner.rap,
	}, true, nil
}

func filterNoLocalMatches(matches []*proto_topic.MatchedSubscription, clientID, publisherClientID string) []*proto_topic.MatchedSubscription {
	if len(matches) == 0 {
		return nil
	}
	out := make([]*proto_topic.MatchedSubscription, 0, len(matches))
	for _, ms := range matches {
		if ms == nil {
			continue
		}
		if publisherClientID != "" && publisherClientID == clientID && ms.GetNoLocal() {
			continue
		}
		out = append(out, ms)
	}
	return out
}

type winnerState struct {
	noLocal     bool
	rap         bool
	qos         int32
	wildcardCnt int
	depth       int
	filter      string
	set         bool
}

// selectWinnerAndSubIDs selects the winning subscription and collects sub IDs.
func selectWinnerAndSubIDs(normalMatches []*proto_topic.MatchedSubscription) (winnerState, []int32) {
	winner := winnerState{
		noLocal: false,
		rap:     true,
	}
	subIDs := make([]int32, 0, len(normalMatches))
	for _, ms := range normalMatches {
		if ms == nil {
			continue
		}
		if shouldReplaceWinner(ms, &winner) {
			winner.set = true
			winner.qos = ms.GetQoS()
			winner.filter = ms.GetTopicFilter()
			winner.wildcardCnt, winner.depth = wildcardMetrics(winner.filter)
			winner.noLocal = ms.GetNoLocal()
			winner.rap = ms.GetRetainAsPublished()
		}

		sid := ms.GetSubscriptionIdentifier()
		if sid <= 0 {
			continue
		}
		subIDs = append(subIDs, sid)
	}
	return winner, subIDs
}

// shouldReplaceWinner checks if the current match beats the existing winner.
func shouldReplaceWinner(ms *proto_topic.MatchedSubscription, winner *winnerState) bool {
	if ms == nil {
		return false
	}
	if winner == nil || !winner.set {
		return true
	}
	q := ms.GetQoS()
	if q != winner.qos {
		return q > winner.qos
	}
	f := ms.GetTopicFilter()
	wc, d := wildcardMetrics(f)
	if wc != winner.wildcardCnt {
		return wc < winner.wildcardCnt
	}
	if d != winner.depth {
		return d > winner.depth
	}
	return f < winner.filter
}

// buildShareGroupTasks converts aggregated shared subscriptions into tasks.
func buildShareGroupTasks(shareAggMap map[string]*shareAgg, publish *packets.Publish, publisherClientID string) ([]ShareGroupTask, error) {
	if publish == nil {
		return nil, nil
	}
	tasks := make([]ShareGroupTask, 0, len(shareAggMap))
	for _, a := range shareAggMap {
		if a == nil || a.winner == nil || a.shareGroup == "" || a.topicFilter == "" {
			continue
		}
		subIDs := make([]int32, len(a.subIDs))
		copy(subIDs, a.subIDs)
		b, err := jsonMarshal(subIDs)
		if err != nil {
			return nil, fmt.Errorf("marshal shared subscription ids failed: topic=%s share_group=%s: %w", publish.Topic, a.shareGroup, err)
		}

		effQoS := int(a.winner.GetQoS())
		if int(publish.QoS) < effQoS {
			effQoS = int(publish.QoS)
		}
		tasks = append(tasks, ShareGroupTask{
			ShareGroup:      a.shareGroup,
			TopicFilter:     a.topicFilter,
			DeliveryQoS:     effQoS,
			PublishQoS:      int(publish.QoS),
			PublisherClient: publisherClientID,
			SubscriptionIDs: string(b),
			WinnerNoLocal:   a.winner.GetNoLocal(),
			WinnerRAP:       a.winner.GetRetainAsPublished(),
		})
	}
	return tasks, nil
}

// wildcardMetrics calculates wildcard count and depth for a topic filter.
func wildcardMetrics(filter string) (wildCnt int, depth int) {
	for _, ch := range filter {
		if ch == '+' || ch == '#' {
			wildCnt++
		}
	}
	if filter == "" {
		return wildCnt, 0
	}
	depth = 1
	for _, ch := range filter {
		if ch == '/' {
			depth++
		}
	}
	return wildCnt, depth
}

// isBetterWinner compares two shared subscriptions for aggregation winner selection.
func isBetterWinner(ms *proto_topic.MatchedSubscription, currentWinner *proto_topic.MatchedSubscription) bool {
	if ms == nil {
		return false
	}
	if currentWinner == nil {
		return true
	}
	q1, q2 := ms.GetQoS(), currentWinner.GetQoS()
	if q1 != q2 {
		return q1 > q2
	}
	f1, f2 := ms.GetTopicFilter(), currentWinner.GetTopicFilter()
	wc1, d1 := wildcardMetrics(f1)
	wc2, d2 := wildcardMetrics(f2)
	if wc1 != wc2 {
		return wc1 < wc2
	}
	if d1 != d2 {
		return d1 > d2
	}
	return f1 < f2
}
