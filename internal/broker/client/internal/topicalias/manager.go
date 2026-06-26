package topicalias

import (
	"sync"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// TopicAliasManager manages MQTT5 Topic Alias state for a single client connection (both directions).
type TopicAliasManager struct {
	mu sync.RWMutex

	// uplinkAliasToTopic stores inbound (client -> server) alias mapping: alias -> topic.
	uplinkAliasToTopic map[uint16]string

	// downlinkTopicToAlias stores outbound (server -> client) alias mapping: topic -> alias.
	downlinkTopicToAlias map[string]uint16

	// downlinkNext is the last allocated alias value for downlink; next allocation uses downlinkNext+1.
	downlinkNext uint16

	// downlinkFree stores aliases that can be reused after a failed first-announcement write.
	downlinkFree []uint16

	// downlinkMax is the max alias value the client declared it can accept from server (CONNECT TopicAliasMaximum).
	// 0 means disabled.
	downlinkMax uint16
}

func NewTopicAliasManager() *TopicAliasManager {
	return &TopicAliasManager{
		uplinkAliasToTopic:   make(map[uint16]string),
		downlinkTopicToAlias: make(map[string]uint16),
	}
}

// SetDownlinkMax stores client-declared downlink Topic Alias Maximum (CONNECT property).
func (m *TopicAliasManager) SetDownlinkMax(max uint16) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downlinkMax = max
}

func (m *TopicAliasManager) GetDownlinkMax() uint16 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.downlinkMax
}

// ResetDownlink clears outbound alias mapping and allocation counter for a new CONNECT on this connection.
func (m *TopicAliasManager) ResetDownlink() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downlinkTopicToAlias = make(map[string]uint16)
	m.downlinkNext = 0
	m.downlinkFree = nil
}

// ApplyDownlink applies MQTT5 Topic Alias rules for outbound PUBLISH (server -> client).
//
// Policy (mirrors existing behavior):
// - If client CONNECT TopicAliasMaximum == 0: do not use Topic Alias, and strip any existing TopicAlias property.
// - Otherwise:
//   - First time a topic is seen (and alias capacity allows): send TopicName + TopicAlias to establish mapping on client.
//   - Subsequent sends for same topic: send TopicAlias only, omit TopicName (TopicName="").
//   - If alias capacity is exhausted for a new topic: fall back to sending full TopicName without TopicAlias.
func (m *TopicAliasManager) ApplyDownlink(p *packets.Publish) {
	if m == nil || p == nil {
		return
	}

	// Topic Alias is a per-direction optimization; never forward inbound aliases as-is.
	if p.Properties != nil {
		p.Properties.TopicAlias = nil
	}

	m.mu.RLock()
	maxAlias := m.downlinkMax
	if maxAlias == 0 || p.Topic == "" {
		m.mu.RUnlock()
		return
	}

	// Fast path: topic already has an alias -> send alias only
	if alias, ok := m.downlinkTopicToAlias[p.Topic]; ok {
		m.mu.RUnlock()
		if alias == 0 || alias > maxAlias {
			// Defensive: ignore corrupted state, fall back to full topic.
			return
		}
		if p.Properties == nil {
			p.Properties = &packets.PublishProperties{}
		}
		p.Properties.TopicAlias = &alias
		p.Topic = ""
		return
	}
	m.mu.RUnlock()

	// Slow path: allocate/store a new mapping if within capacity.
	m.mu.Lock()
	defer m.mu.Unlock()

	maxAlias = m.downlinkMax
	if maxAlias == 0 || p.Topic == "" {
		return
	}

	// Re-check in case another goroutine stored it while we were upgrading lock.
	if alias, ok := m.downlinkTopicToAlias[p.Topic]; ok {
		if alias == 0 || alias > maxAlias {
			return
		}
		if p.Properties == nil {
			p.Properties = &packets.PublishProperties{}
		}
		p.Properties.TopicAlias = &alias
		p.Topic = ""
		return
	}

	// Reuse aliases from failed first-announcement writes first.
	if n := len(m.downlinkFree); n > 0 {
		alias := m.downlinkFree[n-1]
		m.downlinkFree = m.downlinkFree[:n-1]
		if alias != 0 && alias <= maxAlias {
			m.downlinkTopicToAlias[p.Topic] = alias
			if p.Properties == nil {
				p.Properties = &packets.PublishProperties{}
			}
			p.Properties.TopicAlias = &alias
			// Keep TopicName non-empty for first mapping establishment.
			return
		}
		// Corrupted free-list entry, continue to sequential allocation.
	}

	// Allocate sequentially from 1..maxAlias.
	if m.downlinkNext >= maxAlias {
		// Exhausted: do not alias new topics.
		return
	}
	m.downlinkNext++
	alias := m.downlinkNext
	m.downlinkTopicToAlias[p.Topic] = alias

	if p.Properties == nil {
		p.Properties = &packets.PublishProperties{}
	}
	p.Properties.TopicAlias = &alias
	// Keep TopicName non-empty for first mapping establishment.
}

// RevertDownlinkAlias 取消之前为 topic 分配的下行 alias 映射。
// 当 ApplyDownlink 已经向 packet 写入了"首次 announce"信息但写出失败 / 被丢弃时，
// 必须调用本方法回滚，否则后续 fast-path 会把该 alias 当作"已 announce"，
// 客户端会收到 alias-only 但从未见过对应的 TopicName。
//
// 注意：只回滚 next-allocation 类型的 alias（没有 announce 历史）。
// 已经多次成功投递过的 alias 即便偶发一次写失败也不需要回滚。
func (m *TopicAliasManager) RevertDownlinkAlias(topic string) {
	if m == nil || topic == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	alias, ok := m.downlinkTopicToAlias[topic]
	if !ok {
		return
	}
	delete(m.downlinkTopicToAlias, topic)
	// 回收到 free-list，供后续首次 announce 失败回滚后的 topic 复用。
	// 只在映射存在时回收，避免重复回滚产生重复 alias。
	if alias > 0 {
		m.downlinkFree = append(m.downlinkFree, alias)
	}
}

// ApplyUplink applies MQTT5 Topic Alias rules for inbound PUBLISH (client -> server).
//
// Rules (MQTT5):
// - If Topic Alias is present:
//   - alias MUST be > 0.
//   - alias MUST be <= serverMax (server policy). If serverMax == 0, alias usage is not allowed.
//   - If TopicName is present, store alias->topic.
//   - If TopicName is empty, resolve topic from previously stored alias.
func (m *TopicAliasManager) ApplyUplink(p *packets.Publish, serverMax uint16) (finalTopic string, updated bool, err error) {
	if m == nil || p == nil {
		return "", false, nil
	}

	// No properties or no alias -> no-op.
	if p.Properties == nil || p.Properties.TopicAlias == nil {
		return p.Topic, false, nil
	}

	alias := *p.Properties.TopicAlias
	if alias == 0 {
		return "", false, ErrTopicAliasInvalid
	}

	// Server policy: disallow alias if serverMax == 0, or alias exceeds maximum.
	if serverMax == 0 || alias > serverMax {
		return "", false, ErrTopicAliasInvalid
	}

	// TopicName present: store/update alias mapping.
	if p.Topic != "" {
		m.mu.Lock()
		m.uplinkAliasToTopic[alias] = p.Topic
		m.mu.Unlock()
		p.Properties.TopicAlias = nil
		return p.Topic, false, nil
	}

	// TopicName absent: resolve from previously stored alias.
	m.mu.RLock()
	topic := m.uplinkAliasToTopic[alias]
	m.mu.RUnlock()
	if topic == "" {
		return "", false, ErrTopicAliasNotFound
	}
	p.Topic = topic
	p.Properties.TopicAlias = nil
	return topic, true, nil
}
