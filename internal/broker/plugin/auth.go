package plugin

import (
	"context"
	"errors"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// AuthPlugin 认证插件
// 提供用户名密码认证和权限控制
type AuthPlugin struct {
	users map[string]string // 用户名密码映射
}

// NewAuthPlugin 创建认证插件
func NewAuthPlugin() *AuthPlugin {
	return &AuthPlugin{
		users: map[string]string{
			"admin": "admin123",
			"user":  "user123",
		},
	}
}

// OnReceivedConnect 实现连接认证
func (a *AuthPlugin) OnReceivedConnect(ctx context.Context, clientID string, connect *packets.Connect) error {
	// 如果没有用户名密码，允许连接（匿名连接）
	if !connect.UsernameFlag || !connect.PasswordFlag {
		return nil
	}

	// 验证用户名密码
	username := string(connect.Username)
	password := string(connect.Password)

	if expectedPassword, exists := a.users[username]; !exists || expectedPassword != password {
		return errors.New("authentication failed: invalid username or password")
	}

	return nil
}

// OnSubscribe 实现订阅权限控制
func (a *AuthPlugin) OnSubscribe(ctx context.Context, clientID string, subscribe *packets.Subscribe) error {
	// 这里可以实现基于用户名的订阅权限控制
	// 例如：某些用户只能订阅特定主题

	// 示例：admin用户可以订阅所有主题，user用户只能订阅user/开头的主题
	// 这里需要从context或其他地方获取用户名信息

	return nil
}

// OnReceivedPublish 实现发布权限控制
func (a *AuthPlugin) OnReceivedPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	// 这里可以实现基于用户名的发布权限控制
	// 例如：某些用户只能发布到特定主题

	return nil
}

// 其他方法保持空实现
func (a *AuthPlugin) OnSendConnAck(ctx context.Context, clientID string, connAck *packets.ConnAck) error {
	return nil
}

func (a *AuthPlugin) OnSendSubAck(ctx context.Context, clientID string, subAck *packets.Suback) error {
	return nil
}

func (a *AuthPlugin) OnReceivedUnsubscribe(ctx context.Context, clientID string, unsubscribe *packets.Unsubscribe) error {
	return nil
}

func (a *AuthPlugin) OnSendUnsubAck(ctx context.Context, clientID string, unsubAck *packets.Unsuback) error {
	return nil
}

func (a *AuthPlugin) OnSendPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	return nil
}

func (a *AuthPlugin) OnReceivedPubAck(ctx context.Context, clientID string, pubAck *packets.Puback) error {
	return nil
}

func (a *AuthPlugin) OnSendPubAck(ctx context.Context, clientID string, pubAck *packets.Puback) error {
	return nil
}

func (a *AuthPlugin) OnReceivedPubRel(ctx context.Context, clientID string, pubRel *packets.Pubrel) error {
	return nil
}

func (a *AuthPlugin) OnSendPubRel(ctx context.Context, clientID string, pubRel *packets.Pubrel) error {
	return nil
}

func (a *AuthPlugin) OnReceivedPubRec(ctx context.Context, clientID string, pubRec *packets.Pubrec) error {
	return nil
}

func (a *AuthPlugin) OnSendPubRec(ctx context.Context, clientID string, pubRec *packets.Pubrec) error {
	return nil
}

func (a *AuthPlugin) OnReceivedPubComp(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error {
	return nil
}

func (a *AuthPlugin) OnSendPubComp(ctx context.Context, clientID string, pubComp *packets.Pubcomp) error {
	return nil
}

func (a *AuthPlugin) OnReceivedPingReq(ctx context.Context, clientID string, pingReq *packets.Pingreq) error {
	return nil
}

func (a *AuthPlugin) OnSendPingResp(ctx context.Context, clientID string, pingResp *packets.Pingresp) error {
	return nil
}

func (a *AuthPlugin) OnReceivedDisconnect(ctx context.Context, clientID string, disconnect *packets.Disconnect) error {
	return nil
}
