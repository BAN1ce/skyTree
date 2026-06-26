package raft

import "testing"

func TestClusterCloseIsIdempotent(t *testing.T) {
	var cancelCount int
	c := &Cluster{
		cancel: func() {
			cancelCount++
		},
	}

	if err := c.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if cancelCount != 1 {
		t.Fatalf("cancel should be called once, got %d", cancelCount)
	}
}
