package config

// NOTE: These constants define the string values used in YAML/env config for client delivery stores.
// Keep them centralized to avoid scattered hard-coded strings across the codebase.

const (
	// ClientDeliveryQueueTypeSingleNodeBadger selects local Badger-based delivery queue store (single-node mode).
	ClientDeliveryQueueTypeSingleNodeBadger = "single_node_badger"
	// ClientDeliveryQueueTypeScylla selects ScyllaDB/CQL delivery queue metadata store.
	ClientDeliveryQueueTypeScylla = "scylla"
)

const (
	// ClientDeliveryPayloadTypeSingleNodeBadger selects local Badger payload store.
	ClientDeliveryPayloadTypeSingleNodeBadger = "single_node_badger"
	// ClientDeliveryPayloadTypeScylla selects ScyllaDB/CQL payload store.
	// It currently reuses the Cassandra-compatible implementation.
	ClientDeliveryPayloadTypeScylla = "scylla"
)

const (
	// KeyStoreTypeRedis selects Redis as the KeyStore backend (via Store.Default).
	KeyStoreTypeRedis = "redis"
	// KeyStoreTypeBadger selects Badger as the KeyStore backend (via Store.Default).
	KeyStoreTypeBadger = "badger"
)
