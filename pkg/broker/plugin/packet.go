package plugin

import "github.com/eclipse/paho.golang/packets"

type OnReceivedConnect func(clientID string, connect *packets.Connect) error

type OnSendConnAck func(clientID string, connAck *packets.Connack) error

type OnReceivedAuth func(clientID string, auth *packets.Auth) error

type OnReceivedDisconnect func(clientID string, disconnect *packets.Disconnect) error

type OnSubscribe func(clientID string, subscribe *packets.Subscribe) error

type OnSendSubAck func(clientID string, subAck *packets.Suback) error

type OnUnsubscribe func(clientID string, unsubscribe *packets.Unsubscribe) error

type OnSendUnsubAck func(clientID string, unsubAck *packets.Unsuback) error

type OnReceivedPublish func(clientID string, publish packets.Publish) error

type OnReceivedPubAck func(clientID string, pubAck packets.Puback) error

type OnReceivedPubRel func(clientID string, pubRel packets.Pubrel) error

type OnReceivedPubRec func(clientID string, pubRec packets.Pubrec) error

type OnReceivedPubComp func(clientID string, pubComp packets.Pubcomp) error

type OnSendPublish func(clientID string, publish *packets.Publish) error

type OnSendPubAck func(clientID string, pubAck *packets.Puback) error

type OnSendPubRel func(clientID string, pubRel *packets.Pubrel) error

type OnSendPubRec func(clientID string, pubRec *packets.Pubrec) error

type OnSendPubComp func(clientID string, pubComp *packets.Pubcomp) error

type OnReceivedPingReq func(clientID string, pingReq *packets.Pingreq) error

type OnSendPingResp func(clientID string, pingResp *packets.Pingresp) error

type PacketPlugin struct {
	OnReceivedConnect    []OnReceivedConnect
	OnSendConnAck        []OnSendConnAck
	OnReceivedAuth       []OnReceivedAuth
	OnReceivedDisconnect []OnReceivedDisconnect
	OnSubscribe          []OnSubscribe
	OnSendSubAck         []OnSendSubAck
	OnUnsubscribe        []OnUnsubscribe
	OnSendUnsubAck       []OnSendUnsubAck
	OnReceivedPublish    []OnReceivedPublish
	OnReceivedPubAck     []OnReceivedPubAck
	OnReceivedPubRel     []OnReceivedPubRel
	OnReceivedPubRec     []OnReceivedPubRec
	OnReceivedPubComp    []OnReceivedPubComp
	OnSendPublish        []OnSendPublish
	OnSendPubAck         []OnSendPubAck
	OnSendPubRel         []OnSendPubRel
	OnSendPubRec         []OnSendPubRec
	OnSendPubComp        []OnSendPubComp
	OnReceivedPingReq    []OnReceivedPingReq
	OnSendPingResp       []OnSendPingResp
}
