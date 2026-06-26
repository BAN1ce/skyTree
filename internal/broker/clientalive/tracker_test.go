package clientalive

import (
	"sync"
	"testing"
	"time"
)

func TestTrackerUpdateRecordsAliveAndExpireTime(t *testing.T) {
	tracker := NewTracker()
	now := time.Unix(100, 0)

	tracker.Update("client-a", "owner-a", now, 2*time.Second)

	lastAlive, ok := tracker.LastAlive("client-a")
	if !ok {
		t.Fatal("LastAlive() ok = false, want true")
	}
	if !lastAlive.Equal(now) {
		t.Fatalf("LastAlive() = %s, want %s", lastAlive, now)
	}

	expireAt, ok := tracker.ExpireAt("client-a")
	if !ok {
		t.Fatal("ExpireAt() ok = false, want true")
	}
	wantExpireAt := now.Add(3 * time.Second)
	if !expireAt.Equal(wantExpireAt) {
		t.Fatalf("ExpireAt() = %s, want %s", expireAt, wantExpireAt)
	}
}

func TestTrackerUpdateMovesExistingClientInHeap(t *testing.T) {
	tracker := NewTracker()
	base := time.Unix(100, 0)

	tracker.Update("client-a", "owner-a1", base, time.Second)
	tracker.Update("client-b", "owner-b", base, 2*time.Second)
	tracker.Update("client-a", "owner-a2", base.Add(10*time.Second), time.Second)

	got := tracker.ScanExpired(base.Add(2 * time.Second))
	if contains(got, "client-a") {
		t.Fatalf("ScanExpired() = %v, want updated client-a to remain alive", got)
	}
	if len(got) != 0 {
		t.Fatalf("ScanExpired() = %v, want no expired clients before earliest current expire time", got)
	}

	got = tracker.ScanExpired(base.Add(4 * time.Second))
	if !contains(got, "client-b") {
		t.Fatalf("ScanExpired() = %v, want client-b expired", got)
	}
	if contains(got, "client-a") {
		t.Fatalf("ScanExpired() = %v, want client-a to use latest expire time", got)
	}

	got = tracker.ScanExpired(base.Add(12 * time.Second))
	if len(got) != 1 || got[0].ClientID != "client-a" || got[0].OwnerToken != "owner-a2" {
		t.Fatalf("ScanExpired() = %v, want updated client-a owner", got)
	}
}

func TestTrackerScanExpiredUsesPerClientKeepAlive(t *testing.T) {
	tracker := NewTracker()
	base := time.Unix(100, 0)

	tracker.Update("fast-client", "fast-owner", base, time.Second)
	tracker.Update("slow-client", "slow-owner", base, 10*time.Second)

	got := tracker.ScanExpired(base.Add(2 * time.Second))
	if !contains(got, "fast-client") {
		t.Fatalf("ScanExpired() = %v, want fast-client expired", got)
	}
	if contains(got, "slow-client") {
		t.Fatalf("ScanExpired() = %v, want slow-client to remain alive", got)
	}
}

func TestTrackerScanExpiredStopsAtFirstAliveEntry(t *testing.T) {
	tracker := NewTracker()
	base := time.Unix(100, 0)

	tracker.Update("client-a", "owner-a", base, time.Second)
	tracker.Update("client-b", "owner-b", base.Add(10*time.Second), time.Second)
	tracker.Update("client-c", "owner-c", base.Add(20*time.Second), time.Second)

	got := tracker.ScanExpired(base.Add(2 * time.Second))
	if len(got) != 1 || got[0].ClientID != "client-a" || got[0].OwnerToken != "owner-a" {
		t.Fatalf("ScanExpired() = %v, want only client-a", got)
	}

	if _, ok := tracker.LastAlive("client-b"); !ok {
		t.Fatal("client-b should remain indexed after scan stops")
	}
	if _, ok := tracker.LastAlive("client-c"); !ok {
		t.Fatal("client-c should remain indexed after scan stops")
	}
}

func TestTrackerDeleteIfOwnerRemovesOnlyMatchingOwner(t *testing.T) {
	tracker := NewTracker()
	base := time.Unix(100, 0)

	tracker.Update("client-a", "old-owner", base, time.Second)
	tracker.Update("client-a", "new-owner", base.Add(time.Second), time.Second)
	if tracker.DeleteIfOwner("client-a", "old-owner") {
		t.Fatal("DeleteIfOwner() deleted mismatched owner")
	}

	if _, ok := tracker.LastAlive("client-a"); !ok {
		t.Fatal("LastAlive() ok = false after mismatched DeleteIfOwner, want true")
	}
	if !tracker.DeleteIfOwner("client-a", "new-owner") {
		t.Fatal("DeleteIfOwner() ok = false for matching owner")
	}

	if _, ok := tracker.LastAlive("client-a"); ok {
		t.Fatal("LastAlive() ok = true after DeleteIfOwner, want false")
	}
	got := tracker.ScanExpired(base.Add(3 * time.Second))
	if contains(got, "client-a") {
		t.Fatalf("ScanExpired() = %v, want deleted client excluded", got)
	}
}

func TestTrackerConcurrentAccess(t *testing.T) {
	tracker := NewTracker()
	base := time.Unix(100, 0)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			clientID := string(rune('a' + i%10))
			ownerToken := clientID + "-owner"
			tracker.Update(clientID, ownerToken, base.Add(time.Duration(i)*time.Millisecond), time.Second)
			if i%3 == 0 {
				tracker.DeleteIfOwner(clientID, ownerToken)
			}
			_ = tracker.ScanExpired(base.Add(2 * time.Second))
		}()
	}
	wg.Wait()
}

func contains(items []ExpiredClient, want string) bool {
	for _, item := range items {
		if item.ClientID == want {
			return true
		}
	}
	return false
}
