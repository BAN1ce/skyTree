package client

import (
	"testing"
	"time"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// applyMessageExpiryForDelivery 必须只信 ExpiredTime（绝对截止时间），
// 不能因为 publish.Properties.MessageExpiry 被前一次投递写过而延长 deadline。
func TestApplyMessageExpiryUsesAbsoluteDeadlineNotMutatedProperty(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	deadline := now.Add(2 * time.Second).UnixNano()

	expiry := uint32(100) // 残留的旧值（不应被信任）
	pub := &packets.Publish{
		Properties: &packets.PublishProperties{
			MessageExpiry: &expiry,
		},
	}
	msg := &brokerpublish.Message{
		CreatedTime: now.Add(-10 * time.Second).UnixNano(),
		ExpiredTime: deadline,
	}

	if !applyMessageExpiryForDelivery(pub, msg, now) {
		t.Fatal("expected message still deliverable")
	}
	if pub.Properties.MessageExpiry == nil {
		t.Fatal("expected MessageExpiry to be set")
	}
	if got := *pub.Properties.MessageExpiry; got > 2 || got == 0 {
		t.Fatalf("expected remaining ~2s, got %d", got)
	}
}

func TestApplyMessageExpiryDropsExpiredMessage(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	pub := &packets.Publish{Properties: &packets.PublishProperties{}}
	msg := &brokerpublish.Message{ExpiredTime: now.Add(-time.Second).UnixNano()}

	if applyMessageExpiryForDelivery(pub, msg, now) {
		t.Fatal("expected expired message to be dropped")
	}
}

// 没有 ExpiredTime 时不应再回退到相对值；并且应抹除 publish 上残留的 MessageExpiry。
func TestApplyMessageExpiryWithoutExpiredTimeClearsRelativeProperty(t *testing.T) {
	expiry := uint32(99)
	pub := &packets.Publish{
		Properties: &packets.PublishProperties{MessageExpiry: &expiry},
	}
	msg := &brokerpublish.Message{
		CreatedTime: time.Now().Add(-time.Hour).UnixNano(),
		// ExpiredTime 故意为 0
	}

	if !applyMessageExpiryForDelivery(pub, msg, time.Now()) {
		t.Fatal("expected deliverable when ExpiredTime is unset")
	}
	if pub.Properties.MessageExpiry != nil {
		t.Fatalf("expected residual MessageExpiry to be cleared, got %d", *pub.Properties.MessageExpiry)
	}
}
