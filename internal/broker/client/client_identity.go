package client

import (
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/packetpool"
	"github.com/BAN1ce/skyTree/pkg/retry"
)

func (c *Client) GetID() string {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return c.getID()
}

func (c *Client) getID() string {
	return c.ID

}

// GetOwnerToken returns the fencing token for the current active connection.
func (c *Client) GetOwnerToken() string {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.getOwnerToken()
}

func (c *Client) getOwnerToken() string {
	return c.ownerToken
}

// setOwnerToken sets the fencing token for the current active connection.
func (c *Client) setOwnerToken(token string) {
	c.ownerToken = token
}

func (c *Client) MetaString() string {
	return c.metaString()
}

// metaString returns a string for logging without acquiring client locks.
func (c *Client) metaString() string {
	if c == nil {
		return "clientID: <nil> remoteAddr: <nil>"
	}
	var (
		s strings.Builder
	)
	s.WriteString("clientID: ")
	s.WriteString(c.ID)
	s.WriteString(" ")
	s.WriteString("remoteAddr: ")
	if c.conn != nil && c.conn.RemoteAddr() != nil {
		s.WriteString(c.conn.RemoteAddr().String())
	} else {
		s.WriteString("<nil>")
	}
	return s.String()
}

func (c *Client) GetKeepAliveTime() time.Duration {
	return c.keepAlive
}

func (c *Client) KeepAliveExpired(now time.Time) bool {
	if c == nil || c.keepAlive <= 0 {
		return false
	}
	lastAlive, ok := c.aliveTime.Load().(time.Time)
	if !ok || lastAlive.IsZero() {
		return false
	}
	return now.Sub(lastAlive) > c.keepAlive+c.keepAlive/2
}

// IdleDurationSince 返回相对 now 的连接空闲时长（since 上一次 alive 触发）。
// 主要用于排障和 DISCONNECT ReasonString 携带。如果尚未记录 aliveTime，返回 0。
func (c *Client) IdleDurationSince(now time.Time) time.Duration {
	if c == nil {
		return 0
	}
	lastAlive, ok := c.aliveTime.Load().(time.Time)
	if !ok || lastAlive.IsZero() {
		return 0
	}
	if now.Before(lastAlive) {
		return 0
	}
	return now.Sub(lastAlive)
}

func (c *Client) NextPacketID() uint16 {
	if c == nil || c.packetIDFactory == nil {
		return 0
	}
	for attempts := 0; attempts < 65535; attempts++ {
		packetID := c.packetIDFactory.NextPacketID()
		if packetID == 0 {
			continue
		}
		if c.outgoingInflight != nil {
			if _, ok := c.outgoingInflight.Get(packetID); ok {
				continue
			}
		}
		return packetID
	}
	return 0
}

func (c *Client) RetrySend(message *brokerpublish.Message) error {
	if c == nil {
		logger.Logger.Warn().Msg("skip retry send: client is nil")
		return nil
	}
	if message == nil {
		logger.Logger.Warn().Str("client", c.ID).Msg("skip retry send: message is nil")
		return nil
	}
	if message.Publish == nil {
		logger.Logger.Warn().Str("client", c.ID).Msg("skip retry send: publish is nil")
		return nil
	}
	if !message.PubReceived && message.ControlPacket == nil {
		logger.Logger.Warn().Str("client", c.ID).Msg("skip retry send: control packet is nil")
		return nil
	}

	var topic = message.Publish.Topic

	defer func() {
		metric.ExecuteRetryTask.Inc()
		metric.RecordPublishRetryAction(metric.PublishRetryActionExecute)
	}()

	// Increment retry counter.
	if message.RetryInfo != nil {
		message.RetryInfo.Times.Add(1)
		logger.Logger.Info().Int32("retry_count", message.RetryInfo.Times.Load()).Msg("Client retry publish")
	} else {
		logger.Logger.Info().Msg("Client retry publish")
	}
	// find the topic instance, and resend

	var (
		pubrel = &packets.Pubrel{
			PacketID:   message.Publish.PacketID,
			ReasonCode: 0,
		}
	)
	var sendErr error
	if message.PubReceived {
		pubRel := packetpool.PubRelPool.Get()
		defer packetpool.PubRelPool.Put(pubRel)
		pubRel.Content = pubrel
		sendErr = c.RetryWrite(&clientcap.WritePacket{
			Packet: pubRel,
		})
	} else {
		if message.HasSendOnce {
			message.Publish.Duplicate = true
		}
		sendErr = c.Write(&clientcap.WritePacket{
			Packet:    message.ControlPacket,
			FullTopic: topic,
		})
	}

	if sendErr != nil {
		if c.component == nil || c.component.publishRetry == nil {
			logger.Logger.Error().Msg("WithRetryClient: publish retry schedule is not configured")
		} else if message.RetryInfo != nil {
			retrySchedule := c.component.publishRetry
			if err := retrySchedule.Create(retry.NewTask(message.RetryInfo.Key, message, c.ID, message.RetryInfo.IntervalTime)); err != nil {
				logger.Logger.Error().Err(err).Msg("WithRetryClient: publish retry failed")
			}
		}
	}
	return sendErr

}
