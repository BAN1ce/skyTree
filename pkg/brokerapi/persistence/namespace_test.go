package persistence

import "testing"

func TestKeyStoreNamespacesKeepLegacyKeys(t *testing.T) {
	tests := []struct {
		name string
		ns   KeyNamespace
		key  string
	}{
		{name: "retain", ns: KeyNamespaceRetain, key: "skyTree/retain"},
		{name: "keepalive", ns: KeyNamespaceKeepAlive, key: "skyTree/client_alive_time"},
		{name: "node_meta", ns: KeyNamespaceNodeMeta, key: "node_meta/"},
		{name: "acl", ns: KeyNamespaceACLRules, key: "acl:rules"},
	}

	for _, tt := range tests {
		if tt.ns.Key != tt.key {
			t.Fatalf("%s key changed: %q", tt.name, tt.ns.Key)
		}
		if tt.ns.Owner == "" {
			t.Fatalf("%s owner is empty", tt.name)
		}
		if tt.ns.PhysicalCluster != "key_store" {
			t.Fatalf("%s physical cluster = %q, want key_store", tt.name, tt.ns.PhysicalCluster)
		}
	}
}

func TestKeyNamespaceKeyBytesReturnsCopy(t *testing.T) {
	key := KeyNamespaceRetain.KeyBytes()
	key[0] = 'X'

	if KeyNamespaceRetain.Key != "skyTree/retain" {
		t.Fatalf("namespace key mutated: %q", KeyNamespaceRetain.Key)
	}
}
