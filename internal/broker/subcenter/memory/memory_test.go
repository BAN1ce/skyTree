package memory

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
	"github.com/google/uuid"
)

func BenchmarkSub(b *testing.B) {
	center := NewMemorySubCenter()
	for i := 0; i < b.N; i++ {
		_, err := center.CreateSub(context.TODO(), &proto2.SubRequest{
			Topics:   randSubOption(),
			ClientID: uuid.NewString(),
		})
		if err != nil {
			b.Errorf("CreateSub err: %v", err)
		}
	}
}

func randSubOption() []*proto2.SubOption {
	return []*proto2.SubOption{
		{
			Topic: randomTopic(),
			QoS:   1,
		},
		{
			Topic: randomTopic(),
			QoS:   1,
		},
	}
}

func randomTopic() string {
	return fmt.Sprintf("/topic/%d/%d/%d/%d", rand.IntN(999999999), rand.IntN(999999999), rand.IntN(999999999), rand.IntN(999999999))
}

func TestDeleteSub_ReturnsOneAckPerRequestedTopic(t *testing.T) {
	center := NewMemorySubCenter()
	_, err := center.CreateSub(context.TODO(), &proto2.SubRequest{
		ClientID: "client-a",
		Topics: []*proto2.SubOption{
			{Topic: "a/one", QoS: 1},
			{Topic: "a/two", QoS: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateSub: %v", err)
	}

	resp, err := center.DeleteSub(context.TODO(), &proto2.UnSubRequest{
		ClientID: "client-a",
		Topics:   []string{"a/one", "a/two", "a/missing"},
	})
	if err != nil {
		t.Fatalf("DeleteSub: %v", err)
	}

	want := []int32{0, 0, 0x11}
	if len(resp.Topics) != len(want) {
		t.Fatalf("expected %d unsubscribe reasons, got %d: %v", len(want), len(resp.Topics), resp.Topics)
	}
	for idx, got := range resp.Topics {
		if got != want[idx] {
			t.Fatalf("reason[%d]: want %d, got %d", idx, want[idx], got)
		}
	}
}

func TestDeleteSub_MissingShardReturnsNoSubscriptionExisted(t *testing.T) {
	center := NewMemorySubCenter()
	resp, err := center.DeleteSub(context.TODO(), &proto2.UnSubRequest{
		ClientID: "client-a",
		Topics:   []string{"a/missing"},
	})
	if err != nil {
		t.Fatalf("DeleteSub: %v", err)
	}
	if len(resp.Topics) != 1 || resp.Topics[0] != 0x11 {
		t.Fatalf("expected No Subscription Existed, got %v", resp.Topics)
	}
}

func TestGetShareGroupMembersReturnsStableOrder(t *testing.T) {
	center := NewMemorySubCenter()
	for _, sub := range []struct {
		clientID string
		topic    string
	}{
		{clientID: "client-c", topic: "$share/g/z"},
		{clientID: "client-a", topic: "$share/g/b"},
		{clientID: "client-a", topic: "$share/g/a"},
		{clientID: "client-b", topic: "$share/g/a"},
	} {
		_, err := center.CreateSub(context.TODO(), &proto2.SubRequest{
			ClientID: sub.clientID,
			Topics: []*proto2.SubOption{
				{Topic: sub.topic, QoS: 1},
			},
		})
		if err != nil {
			t.Fatalf("CreateSub %s/%s: %v", sub.clientID, sub.topic, err)
		}
	}

	for range 20 {
		resp, err := center.GetShareGroupMembers(context.TODO(), &proto2.GetShareGroupMembersRequest{ShareGroup: "g"})
		if err != nil {
			t.Fatalf("GetShareGroupMembers: %v", err)
		}
		got := make([]string, 0, len(resp.GetMembers()))
		for _, member := range resp.GetMembers() {
			got = append(got, member.GetClientID()+"|"+member.GetTopicFilter())
		}
		want := append([]string(nil), got...)
		sort.Strings(want)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("members not sorted: got %v, want %v", got, want)
			}
		}
	}
}
