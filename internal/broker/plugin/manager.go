package plugin

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/logger"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func (p *Plugins) DoReceivedConnect(ctx context.Context, clientID string, connect *packets.Connect) error {
	if p.OnReceivedConnect == nil {
		return nil
	}
	for _, f := range p.OnReceivedConnect {
		if err := f(ctx, clientID, connect); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedConnect error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendConnAck(ctx context.Context, clientID string, connAck *packets.ConnAck) error {
	if p.OnSendConnAck == nil {
		return nil
	}
	for _, f := range p.OnSendConnAck {
		if err := f(ctx, clientID, connAck); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendConnAck error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoReceivedDisconnect(ctx context.Context, clientID string, disconnect *packets.Disconnect) error {
	if p.OnReceivedDisconnect == nil {
		return nil
	}
	for _, f := range p.OnReceivedDisconnect {
		if err := f(ctx, clientID, disconnect); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedDisconnect error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoReceivedSubscribe(ctx context.Context, clientID string, subscribe *packets.Subscribe) error {
	if p.OnSubscribe == nil {
		return nil
	}
	for _, f := range p.OnSubscribe {
		if err := f(ctx, clientID, subscribe); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSubscribe error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendSubAck(ctx context.Context, clientID string, subAck *packets.Suback) error {
	if p.OnSendSubAck == nil {
		return nil
	}
	for _, f := range p.OnSendSubAck {
		if err := f(ctx, clientID, subAck); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendSubAck error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoReceivedUnsubscribe(ctx context.Context, clientID string, unsubscribe *packets.Unsubscribe) error {
	if p.OnUnsubscribe == nil {
		return nil
	}
	for _, f := range p.OnUnsubscribe {
		if err := f(ctx, clientID, unsubscribe); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnUnsubscribe error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendUnsubAck(ctx context.Context, clientID string, unsubAck *packets.Unsuback) error {
	if p.OnSendUnsubAck == nil {
		return nil
	}
	for _, f := range p.OnSendUnsubAck {
		if err := f(ctx, clientID, unsubAck); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendUnsubAck error")
			return err
		}
	}
	return nil
}

// 发布相关插件执行
func (p *Plugins) DoReceivedPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	if p.OnReceivedPublish == nil {
		return nil
	}
	for _, f := range p.OnReceivedPublish {
		if err := f(ctx, clientID, publish); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedPublish error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	if p.OnSendPublish == nil {
		return nil
	}
	for _, f := range p.OnSendPublish {
		if err := f(ctx, clientID, publish); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendPublish error")
			return err
		}
	}
	return nil
}

// QoS确认相关插件执行
func (p *Plugins) DoReceivedPubAck(ctx context.Context, clientID string, pubAck *packets.Puback) error {
	if p.OnReceivedPubAck == nil {
		return nil
	}
	for _, f := range p.OnReceivedPubAck {
		if err := f(ctx, clientID, pubAck); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedPubAck error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendPubAck(ctx context.Context, clientID string, pubAck *packets.Puback) error {
	if p.OnSendPubAck == nil {
		return nil
	}
	for _, f := range p.OnSendPubAck {
		if err := f(ctx, clientID, pubAck); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendPubAck error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoReceivedPubRel(ctx context.Context, clientID string, pubRel *packets.Pubrel) error {
	if p.OnReceivedPubRel == nil {
		return nil
	}
	for _, f := range p.OnReceivedPubRel {
		if err := f(ctx, clientID, pubRel); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedPubRel error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendPubRel(ctx context.Context, clientID string, pubRel *packets.Pubrel) error {
	if p.OnSendPubRel == nil {
		return nil
	}
	for _, f := range p.OnSendPubRel {
		if err := f(ctx, clientID, pubRel); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendPubRel error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoReceivedPubRec(ctx context.Context, clientID string, pubRec *packets.Pubrec) error {
	if p.OnReceivedPubRec == nil {
		return nil
	}
	for _, f := range p.OnReceivedPubRec {
		if err := f(ctx, clientID, pubRec); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedPubRec error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendPubRec(ctx context.Context, clientID string, pubRec *packets.Pubrec) error {
	if p.OnSendPubRec == nil {
		return nil
	}
	for _, f := range p.OnSendPubRec {
		if err := f(ctx, clientID, pubRec); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendPubRec error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoReceivedPubComp(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error {
	if p.OnReceivedPubComp == nil {
		return nil
	}
	for _, f := range p.OnReceivedPubComp {
		if err := f(ctx, clientID, pubComp); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedPubComp error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendPubComp(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error {
	if p.OnSendPubComp == nil {
		return nil
	}
	for _, f := range p.OnSendPubComp {
		if err := f(ctx, clientID, pubComp); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendPubComp error")
			return err
		}
	}
	return nil
}

// 心跳相关插件执行
func (p *Plugins) DoReceivedPingReq(ctx context.Context, clientID string, pingReq *packets.Pingreq) error {
	if p.OnReceivedPingReq == nil {
		return nil
	}
	for _, f := range p.OnReceivedPingReq {
		if err := f(ctx, clientID, pingReq); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedPingReq error")
			return err
		}
	}
	return nil
}

func (p *Plugins) DoSendPingResp(ctx context.Context, clientID string, pingResp *packets.Pingresp) error {
	if p.OnSendPingResp == nil {
		return nil
	}
	for _, f := range p.OnSendPingResp {
		if err := f(ctx, clientID, pingResp); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendPingResp error")
			return err
		}
	}
	return nil
}

// AUTH相关插件执行
func (p *Plugins) DoReceivedAuth(ctx context.Context, clientID string, auth *packets.Auth) (*packets.Auth, error) {
	if p.OnReceivedAuth == nil || len(p.OnReceivedAuth) == 0 {
		return auth, nil // 如果没有插件，直接返回原报文
	}

	var result *packets.Auth = auth
	for _, f := range p.OnReceivedAuth {
		var err error
		result, err = f(ctx, clientID, result)
		if err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnReceivedAuth error")
			return nil, err
		}
		if result == nil {
			logger.Logger.Error().Str("client", clientID).Msg("plugin OnReceivedAuth returned nil auth packet")
			return nil, fmt.Errorf("plugin returned nil auth packet")
		}
	}
	return result, nil
}

func (p *Plugins) DoSendAuth(ctx context.Context, clientID string, auth *packets.Auth) error {
	if p.OnSendAuth == nil {
		return nil
	}
	for _, f := range p.OnSendAuth {
		if err := f(ctx, clientID, auth); err != nil {
			logger.Logger.Error().Str("client", clientID).Err(err).Msg("plugin OnSendAuth error")
			return err
		}
	}
	return nil
}

// 客户端错误处理插件执行
func (p *Plugins) DoClientError(ctx context.Context, clientID string, err error) error {
	if p.OnClientError == nil {
		return nil
	}
	for _, f := range p.OnClientError {
		if pluginErr := f(ctx, clientID, err); pluginErr != nil {
			logger.Logger.Error().Str("client", clientID).Err(pluginErr).Msg("plugin OnClientError error")
			return pluginErr
		}
	}
	return nil
}
