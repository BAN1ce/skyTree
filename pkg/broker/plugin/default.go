package plugin

import (
	"github.com/BAN1ce/skyTree/inner/metric"
	"github.com/eclipse/paho.golang/packets"
)

var (
	defaultOnReceivedConnect = []OnReceivedConnect{
		func(clientID string, connect *packets.Connect) error {
			metric.ConnectReceived.Add(1)
			return nil
		},
	}

	defaultOnSendConnAck = []OnSendConnAck{
		func(clientID string, connAck *packets.Connack) error {
			metric.ConnectAckSend.Add(1)
			return nil
		},
	}

	defaultOnReceivedAuth = []OnReceivedAuth{
		func(clientID string, auth *packets.Auth) error {
			metric.AuthReceived.Add(1)
			return nil
		},
	}

	defaultOnReceivedDisconnect = []OnReceivedDisconnect{
		func(clientID string, disconnect *packets.Disconnect) error {
			metric.DisconnectReceived.Add(1)
			return nil
		},
	}

	defaultOnReceivedSubscribe = []OnSubscribe{
		func(clientID string, subscribe *packets.Subscribe) error {
			metric.SubscribeReceived.Add(1)
			return nil
		},
	}

	defaultOnSendSubAck = []OnSendSubAck{
		func(clientID string, subAck *packets.Suback) error {
			metric.SubscribeAckSend.Add(1)
			return nil
		},
	}

	defaultOnReceivedUnsubscribe = []OnUnsubscribe{
		func(clientID string, unsubscribe *packets.Unsubscribe) error {
			metric.UnsubscribeReceived.Add(1)
			return nil
		},
	}

	defaultOnSendUnsubAck = []OnSendUnsubAck{
		func(clientID string, unsubAck *packets.Unsuback) error {
			metric.UnsubscribeAckSend.Add(1)
			return nil
		},
	}

	defaultOnReceivedPublish = []OnReceivedPublish{
		func(clientID string, publish packets.Publish) error {
			metric.PublishReceived.Add(1)
			return nil
		},
	}

	defaultOnReceivedPubAck = []OnReceivedPubAck{
		func(clientID string, pubAck packets.Puback) error {
			metric.PublishAckReceived.Add(1)
			return nil
		},
	}

	defaultOnReceivedPubRel = []OnReceivedPubRel{
		func(clientID string, pubRel packets.Pubrel) error {
			metric.PubRelReceived.Add(1)
			return nil
		},
	}

	defaultOnReceivedPubRec = []OnReceivedPubRec{
		func(clientID string, pubRec packets.Pubrec) error {
			metric.PubRecReceived.Add(1)
			return nil
		},
	}

	defaultOnReceivedPubComp = []OnReceivedPubComp{
		func(clientID string, pubComp packets.Pubcomp) error {
			metric.PubCompReceived.Add(1)
			return nil
		},
	}

	defaultOnSendPublish = []OnSendPublish{
		func(clientID string, publish *packets.Publish) error {
			metric.PublishSend.Add(1)
			return nil
		},
	}

	defaultOnSendPubAck = []OnSendPubAck{
		func(clientID string, pubAck *packets.Puback) error {
			metric.PublishAckSend.Add(1)
			return nil
		},
	}

	defaultOnSendPubRel = []OnSendPubRel{
		func(clientID string, pubRel *packets.Pubrel) error {
			metric.PubRelSend.Add(1)
			return nil
		},
	}

	defaultOnSendPubRec = []OnSendPubRec{
		func(clientID string, pubRec *packets.Pubrec) error {
			metric.PubRecSend.Add(1)
			return nil
		},
	}

	defaultOnSendPubComp = []OnSendPubComp{
		func(clientID string, pubComp *packets.Pubcomp) error {
			metric.PubCompSend.Add(1)
			return nil
		},
	}

	defaultOnReceivedPingReq = []OnReceivedPingReq{
		func(clientID string, pingReq *packets.Pingreq) error {
			metric.PingReceived.Add(1)
			return nil
		},
	}

	defaultOnSendPingResp = []OnSendPingResp{
		func(clientID string, pingResp *packets.Pingresp) error {
			metric.PongSend.Add(1)
			return nil
		},
	}
)

func NewDefaultPlugin() *Plugins {
	return &Plugins{
		PacketPlugin: PacketPlugin{
			OnReceivedConnect: defaultOnReceivedConnect,
			OnSendConnAck:     defaultOnSendConnAck,

			OnReceivedAuth:       defaultOnReceivedAuth,
			OnReceivedDisconnect: defaultOnReceivedDisconnect,

			// about subscribe
			OnSubscribe:  defaultOnReceivedSubscribe,
			OnSendSubAck: defaultOnSendSubAck,

			// about unsubscribe
			OnUnsubscribe:  defaultOnReceivedUnsubscribe,
			OnSendUnsubAck: defaultOnSendUnsubAck,

			// about publish
			OnReceivedPublish: defaultOnReceivedPublish,
			OnReceivedPubAck:  defaultOnReceivedPubAck,

			OnReceivedPubRel:  defaultOnReceivedPubRel,
			OnReceivedPubRec:  defaultOnReceivedPubRec,
			OnReceivedPubComp: defaultOnReceivedPubComp,

			OnSendPublish: defaultOnSendPublish,
			OnSendPubAck:  defaultOnSendPubAck,
			OnSendPubRel:  defaultOnSendPubRel,
			OnSendPubRec:  defaultOnSendPubRec,
			OnSendPubComp: defaultOnSendPubComp,

			// about ping pong
			OnReceivedPingReq: defaultOnReceivedPingReq,
			OnSendPingResp:    defaultOnSendPingResp,
		},
		ClientPlugin: ClientPlugin{},
	}

}
