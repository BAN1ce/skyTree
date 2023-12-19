package plugin

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

const (
	PluginOnReceivedConnect = iota
	PluginOnSendConnAck
	PluginOnReceivedAuth
	PluginOnReceivedDisconnect
	PluginOnSubscribe
	PluginOnSendSubAck
	PluginOnUnsubscribe
	PluginOnSendUnsubAck
	PluginOnReceivedPublish
	PluginOnReceivedPubAck
	PluginOnReceivedPubRel
	PluginOnReceivedPubRec
	PluginOnReceivedPubComp
	PluginOnSendPublish
	PluginOnSendPubAck
	PluginOnSendPubRel
	PluginOnSendPubRec
	PluginOnSendPubComp
	PluginOnReceivedPingReq
	PluginOnSendPingResp
)

//var (
//	m := map[int]map[bool]int{
//		PluginOnReceivedConnect: {
//			true:  PluginOnReceivedConnect,
//			false: PluginOnReceivedConnect,
//		},
//	}
//)
//
//func PacketPluginNumber(received bool, packet packets.Packet) int {
//	switch packet.(type) {
//	case *packets.Connect:
//		return m[PluginOnReceivedConnect][received]
//	case *packets.Connack:
//		return PluginOnSendConnAck
//	case *packets.Auth:
//		return PluginOnReceivedAuth
//	case *packets.Disconnect:
//		return PluginOnReceivedDisconnect
//
//	}
//
//}

type Plugins struct {
	// Packet Plugin
	PacketPlugin

	// Client State

	ClientPlugin
}

func (p *Plugins) DoReceivedConnect(clientID string, connect *packets.Connect) {
	if p.OnReceivedConnect == nil {
		return
	}
	for _, f := range p.OnReceivedConnect {
		if err := f(clientID, connect); err != nil {
			logger.Logger.Error("plugin OnReceivedConnect error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendConnAck(clientID string, connAck *packets.Connack) {
	if p.OnSendConnAck == nil {
		return
	}
	for _, f := range p.OnSendConnAck {
		if err := f(clientID, connAck); err != nil {
			logger.Logger.Error("plugin OnSendConnAck error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedAuth(clientID string, auth *packets.Auth) {
	if p.OnReceivedAuth == nil {
		return
	}
	for _, f := range p.OnReceivedAuth {
		if err := f(clientID, auth); err != nil {
			logger.Logger.Error("plugin OnReceivedAuth error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedDisconnect(clientID string, disconnect *packets.Disconnect) {
	if p.OnReceivedDisconnect == nil {
		return
	}
	for _, f := range p.OnReceivedDisconnect {
		if err := f(clientID, disconnect); err != nil {
			logger.Logger.Error("plugin OnReceivedDisconnect error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedSubscribe(clientID string, subscribe *packets.Subscribe) {
	if p.OnSubscribe == nil {
		return
	}
	for _, f := range p.OnSubscribe {
		if err := f(clientID, subscribe); err != nil {
			logger.Logger.Error("plugin OnSubscribe error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendSubAck(clientID string, subAck *packets.Suback) {
	if p.OnSendSubAck == nil {
		return
	}
	for _, f := range p.OnSendSubAck {
		if err := f(clientID, subAck); err != nil {
			logger.Logger.Error("plugin OnSendSubAck error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedUnsubscribe(clientID string, unsubscribe *packets.Unsubscribe) {
	if p.OnUnsubscribe == nil {
		return
	}
	for _, f := range p.OnUnsubscribe {
		if err := f(clientID, unsubscribe); err != nil {
			logger.Logger.Error("plugin OnUnsubscribe error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendUnsubAck(clientID string, unsubAck *packets.Unsuback) {
	if p.OnSendUnsubAck == nil {
		return
	}
	for _, f := range p.OnSendUnsubAck {
		if err := f(clientID, unsubAck); err != nil {
			logger.Logger.Error("plugin OnSendUnsubAck error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedPublish(clientID string, publish packets.Publish) {
	if p.OnReceivedPublish == nil {
		return
	}
	for _, f := range p.OnReceivedPublish {
		if err := f(clientID, publish); err != nil {
			logger.Logger.Error("plugin OnReceivedPublish error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedPubAck(clientID string, pubAck packets.Puback) {
	if p.OnReceivedPubAck == nil {
		return
	}
	for _, f := range p.OnReceivedPubAck {
		if err := f(clientID, pubAck); err != nil {
			logger.Logger.Error("plugin OnReceivedPubAck error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedPubRel(clientID string, pubRel packets.Pubrel) {
	if p.OnReceivedPubRel == nil {
		return
	}
	for _, f := range p.OnReceivedPubRel {
		if err := f(clientID, pubRel); err != nil {
			logger.Logger.Error("plugin OnReceivedPubRel error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedPubRec(clientID string, pubRec packets.Pubrec) {
	if p.OnReceivedPubRec == nil {
		return
	}
	for _, f := range p.OnReceivedPubRec {
		if err := f(clientID, pubRec); err != nil {
			logger.Logger.Error("plugin OnReceivedPubRec error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedPubComp(clientID string, pubComp packets.Pubcomp) {
	if p.OnReceivedPubComp == nil {
		return
	}
	for _, f := range p.OnReceivedPubComp {
		if err := f(clientID, pubComp); err != nil {
			logger.Logger.Error("plugin OnReceivedPubComp error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendPublish(clientID string, publish *packets.Publish) {
	if p.OnSendPublish == nil {
		return
	}
	for _, f := range p.OnSendPublish {
		if err := f(clientID, publish); err != nil {
			logger.Logger.Error("plugin OnSendPublish error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendPubAck(clientID string, pubAck *packets.Puback) {
	if p.OnSendPubAck == nil {
		return
	}
	for _, f := range p.OnSendPubAck {
		if err := f(clientID, pubAck); err != nil {
			logger.Logger.Error("plugin OnSendPubAck error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendPubRel(clientID string, pubRel *packets.Pubrel) {
	if p.OnSendPubRel == nil {
		return
	}
	for _, f := range p.OnSendPubRel {
		if err := f(clientID, pubRel); err != nil {
			logger.Logger.Error("plugin OnSendPubRel error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendPubRec(clientID string, pubRec *packets.Pubrec) {
	if p.OnSendPubRec == nil {
		return
	}
	for _, f := range p.OnSendPubRec {
		if err := f(clientID, pubRec); err != nil {
			logger.Logger.Error("plugin OnSendPubRec error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendPubComp(clientID string, pubComp *packets.Pubcomp) {
	if p.OnSendPubComp == nil {
		return
	}
	for _, f := range p.OnSendPubComp {
		if err := f(clientID, pubComp); err != nil {
			logger.Logger.Error("plugin OnSendPubComp error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoReceivedPingReq(clientID string, pingReq *packets.Pingreq) {
	if p.OnReceivedPingReq == nil {
		return
	}
	for _, f := range p.OnReceivedPingReq {
		if err := f(clientID, pingReq); err != nil {
			logger.Logger.Error("plugin OnReceivedPingReq error", zap.Error(err), zap.String("client", clientID))
		}
	}
}

func (p *Plugins) DoSendPingResp(clientID string, pingResp *packets.Pingresp) {
	if p.OnSendPingResp == nil {
		return
	}
	for _, f := range p.OnSendPingResp {
		if err := f(clientID, pingResp); err != nil {
			logger.Logger.Error("plugin OnSendPingResp error", zap.Error(err), zap.String("client", clientID))
		}
	}
}
