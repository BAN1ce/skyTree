package client

import packets "github.com/BAN1ce/skyTree/pkg/mqtt5"

func clonePublishForLiveDelivery(p *packets.Publish) *packets.Publish {
	out := clonePublish(p)
	if out == nil {
		return nil
	}
	out.Duplicate = false
	out.Retain = false
	return out
}

func clonePublish(p *packets.Publish) *packets.Publish {
	if p == nil {
		return nil
	}
	out := *p
	if p.Payload != nil {
		out.Payload = append([]byte(nil), p.Payload...)
	}
	out.Properties = clonePublishProperties(p.Properties)
	return &out
}

func clonePublishProperties(p *packets.PublishProperties) *packets.PublishProperties {
	if p == nil {
		return nil
	}
	out := *p
	if p.CorrelationData != nil {
		out.CorrelationData = append([]byte(nil), p.CorrelationData...)
	}
	if p.SubscriptionIdentifier != nil {
		out.SubscriptionIdentifier = append([]int(nil), p.SubscriptionIdentifier...)
	}
	if p.User != nil {
		out.User = append([]packets.User(nil), p.User...)
	}
	return &out
}
