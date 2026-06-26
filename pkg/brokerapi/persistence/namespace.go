package persistence

const (
	KeyStorePhysicalCluster = "key_store"

	KeyNamespaceRetainKey    = GlobalPrefix + "/retain"
	KeyNamespaceKeepAliveKey = GlobalPrefix + "/client_alive_time"
	KeyNamespaceNodeMetaKey  = "node_meta/"
	KeyNamespaceACLRulesKey  = "acl:rules"
)

type KeyNamespaceOwner string

const (
	KeyNamespaceOwnerRetain    KeyNamespaceOwner = "retain"
	KeyNamespaceOwnerKeepAlive KeyNamespaceOwner = "keepalive"
	KeyNamespaceOwnerNodeMeta  KeyNamespaceOwner = "node_meta"
	KeyNamespaceOwnerACL       KeyNamespaceOwner = "acl"
)

type KeyNamespace struct {
	Owner           KeyNamespaceOwner
	PhysicalCluster string
	Key             string
	Notes           string
}

func (n KeyNamespace) KeyBytes() []byte {
	return []byte(n.Key)
}

var (
	KeyNamespaceRetain = KeyNamespace{
		Owner:           KeyNamespaceOwnerRetain,
		PhysicalCluster: KeyStorePhysicalCluster,
		Key:             KeyNamespaceRetainKey,
		Notes:           "MQTT retained messages keyed by topic.",
	}
	KeyNamespaceKeepAlive = KeyNamespace{
		Owner:           KeyNamespaceOwnerKeepAlive,
		PhysicalCluster: KeyStorePhysicalCluster,
		Key:             KeyNamespaceKeepAliveKey,
		Notes:           "Client last-alive timestamps stored as hash fields.",
	}
	KeyNamespaceNodeMeta = KeyNamespace{
		Owner:           KeyNamespaceOwnerNodeMeta,
		PhysicalCluster: KeyStorePhysicalCluster,
		Key:             KeyNamespaceNodeMetaKey,
		Notes:           "Cluster node metadata keyed by node ID.",
	}
	KeyNamespaceACLRules = KeyNamespace{
		Owner:           KeyNamespaceOwnerACL,
		PhysicalCluster: KeyStorePhysicalCluster,
		Key:             KeyNamespaceACLRulesKey,
		Notes:           "ACL rules YAML blob used when file mode is not active.",
	}
)
