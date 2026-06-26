package client

import packets "github.com/BAN1ce/skyTree/pkg/mqtt5"

const (
	mqtt5SubscriptionIdentifierMin = 1
	mqtt5SubscriptionIdentifierMax = 268435455
)

func setPublishSubscriptionIdentifiers(publishContent *packets.Publish, subIDs []int32) {
	if publishContent == nil {
		return
	}
	validSubIDs := sanitizeSubscriptionIdentifiers(subIDs)
	if len(validSubIDs) == 0 {
		if publishContent.Properties != nil {
			publishContent.Properties.SubscriptionIdentifier = nil
		}
		return
	}
	if publishContent.Properties == nil {
		publishContent.Properties = &packets.PublishProperties{}
	}
	publishContent.Properties.SubscriptionIdentifier = validSubIDs
}

func sanitizeSubscriptionIdentifiers(subIDs []int32) []int {
	if len(subIDs) == 0 {
		return nil
	}
	clean := make([]int, 0, len(subIDs))
	for _, sid := range subIDs {
		if sid < mqtt5SubscriptionIdentifierMin || sid > mqtt5SubscriptionIdentifierMax {
			continue
		}
		clean = append(clean, int(sid))
	}
	return clean
}
