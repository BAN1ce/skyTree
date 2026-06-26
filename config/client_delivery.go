package config

import (
	"fmt"
	"strings"
)

// ClientDeliverySpec is the resolved configuration for building the client-centric delivery store.
// It splits the delivery queue metadata store and the message payload store.
type ClientDeliverySpec struct {
	QueueType   string
	PayloadType string
}

// ResolveClientDeliverySpec resolves the effective delivery queue/payload store selection.
// Both queue/payload drivers must be explicitly configured.
func (cfg AppConfig) ResolveClientDeliverySpec() (ClientDeliverySpec, error) {
	queueType, payloadType := cfg.trimmedDeliveryTypes()
	spec := cfg.buildClientDeliverySpec(queueType, payloadType)
	return spec, cfg.validateClientDeliverySpec(spec)
}

func (cfg AppConfig) trimmedDeliveryTypes() (queueType string, payloadType string) {
	return strings.TrimSpace(cfg.Storage.DeliveryQueue.Type), strings.TrimSpace(cfg.Storage.Payload.Type)
}

func (cfg AppConfig) buildClientDeliverySpec(queueType string, payloadType string) ClientDeliverySpec {
	return ClientDeliverySpec{
		QueueType:   queueType,
		PayloadType: payloadType,
	}
}

func (cfg AppConfig) validateClientDeliverySpec(spec ClientDeliverySpec) error {
	if spec.QueueType == "" {
		return fmt.Errorf("storage.delivery_queue.driver is required (supported: %s, %s)", ClientDeliveryQueueTypeSingleNodeBadger, ClientDeliveryQueueTypeScylla)
	}
	if spec.PayloadType == "" {
		return fmt.Errorf("storage.payload.driver is required (supported: %s, %s)", ClientDeliveryPayloadTypeSingleNodeBadger, ClientDeliveryPayloadTypeScylla)
	}

	switch spec.QueueType {
	case ClientDeliveryQueueTypeSingleNodeBadger:
		if cfg.Cluster.Enable {
			return fmt.Errorf("delivery queue type=%s is only supported when cluster.enable=false", ClientDeliveryQueueTypeSingleNodeBadger)
		}
	case ClientDeliveryQueueTypeScylla:
	default:
		return fmt.Errorf(
			"unsupported delivery queue store type=%q (supported: %s, %s)",
			spec.QueueType,
			ClientDeliveryQueueTypeSingleNodeBadger,
			ClientDeliveryQueueTypeScylla,
		)
	}

	switch spec.PayloadType {
	case ClientDeliveryPayloadTypeSingleNodeBadger, ClientDeliveryPayloadTypeScylla:
	default:
		return fmt.Errorf(
			"unsupported payload store type=%q (supported: %s, %s)",
			spec.PayloadType,
			ClientDeliveryPayloadTypeSingleNodeBadger,
			ClientDeliveryPayloadTypeScylla,
		)
	}

	if spec.QueueType != spec.PayloadType {
		return fmt.Errorf(
			"delivery queue/payload driver mismatch: queue=%q payload=%q (supported pairs: %s+%s, %s+%s)",
			spec.QueueType,
			spec.PayloadType,
			ClientDeliveryQueueTypeSingleNodeBadger,
			ClientDeliveryPayloadTypeSingleNodeBadger,
			ClientDeliveryQueueTypeScylla,
			ClientDeliveryPayloadTypeScylla,
		)
	}
	return nil
}
