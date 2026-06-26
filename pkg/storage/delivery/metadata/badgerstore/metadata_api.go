package badgerstore

// LocalDeliveryMetadataStore stores delivery metadata (tasks/cursors) in local Badger.
type LocalDeliveryMetadataStore = LocalDeliveryQueueStore

func NewLocalDeliveryMetadataStore(basePath string, nodeID uint64) (*LocalDeliveryMetadataStore, error) {
	return NewLocalDeliveryQueueStore(basePath, nodeID)
}
