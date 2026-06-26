package raft

import "testing"

func TestSystemClusterDescriptors(t *testing.T) {
	descriptors := SystemClusterDescriptors()
	if len(descriptors) != 4 {
		t.Fatalf("expected 4 system clusters, got %d", len(descriptors))
	}

	if descriptors[0].ClusterID != ClusterIDSubCenter || descriptors[0].Name != "sub_center" {
		t.Fatalf("unexpected first descriptor: %+v", descriptors[0])
	}

	if ClusterName(ClusterIDKeyStore) != "key_store" {
		t.Fatalf("unexpected key store cluster name")
	}

	ids := SystemClusterIDs()
	if len(ids) != len(descriptors) {
		t.Fatalf("expected %d ids, got %d", len(descriptors), len(ids))
	}
	for i, descriptor := range descriptors {
		if ids[i] != descriptor.ClusterID {
			t.Fatalf("id %d = %d, want %d", i, ids[i], descriptor.ClusterID)
		}
	}
}

func TestClusterNameFallbackForUnknownClusterID(t *testing.T) {
	const unknownClusterID uint64 = 1007
	if ClusterName(unknownClusterID) != "cluster_1007" {
		t.Fatalf("unexpected fallback cluster name: %q", ClusterName(unknownClusterID))
	}
}
