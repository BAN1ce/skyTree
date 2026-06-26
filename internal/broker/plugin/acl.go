package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/BAN1ce/skyTree/internal/broker/acl"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
)

// ACLPlugin enforces topic-level permissions on SUBSCRIBE and PUBLISH.
// Rules are loaded from local file (preferred when exists) otherwise from KeyStore.
type ACLPlugin struct {
	manager          *acl.Manager
	usernameByClient sync.Map // clientID -> username
}

func NewACLPlugin(keyStore store.KVStore, filePath string, keyStoreKey string, defaultDeny bool) *ACLPlugin {
	mgr := acl.NewManager(acl.ManagerConfig{
		Store:       keyStore,
		KeyStoreKey: keyStoreKey,
		FilePath:    filePath,
		DefaultDeny: defaultDeny,
	})
	return &ACLPlugin{
		manager: mgr,
	}
}

func NewACLPluginWithManager(manager *acl.Manager) *ACLPlugin {
	return &ACLPlugin{manager: manager}
}

func (p *ACLPlugin) OnReceivedConnect(ctx context.Context, clientID string, connect *packets.Connect) error {
	if p == nil || clientID == "" || connect == nil {
		return nil
	}
	username := ""
	if connect.UsernameFlag && len(connect.Username) > 0 {
		username = string(connect.Username)
	}
	p.usernameByClient.Store(clientID, username)
	if p.manager == nil {
		return fmt.Errorf("acl manager is nil")
	}
	return p.manager.EnsureLoaded(ctx)
}

func (p *ACLPlugin) OnSubscribe(ctx context.Context, clientID string, subscribe *packets.Subscribe) error {
	if p == nil || subscribe == nil {
		return nil
	}
	ev, err := p.evaluator(ctx)
	if err != nil {
		return err
	}
	username := p.username(clientID)

	for _, s := range subscribe.Subscriptions {
		topicFilter := s.Topic
		if topicFilter == "" {
			continue
		}
		// Shared subscription: authorize against the actual topic filter part.
		_, actual := topicutil.ParseShareTopic(topicFilter)
		if actual == "" {
			actual = topicFilter
		}
		allowed, err := ev.AllowSubscribe(ctx, username, clientID, actual)
		if err != nil || !allowed {
			return fmt.Errorf("acl denied subscribe: client=%s topic_filter=%s", clientID, topicFilter)
		}
	}
	return nil
}

func (p *ACLPlugin) OnReceivedPublish(ctx context.Context, clientID string, publish *packets.Publish) error {
	if p == nil || publish == nil {
		return nil
	}
	ev, err := p.evaluator(ctx)
	if err != nil {
		return err
	}
	username := p.username(clientID)
	allowed, err := ev.AllowPublish(ctx, username, clientID, publish.Topic)
	if err != nil || !allowed {
		return fmt.Errorf("acl denied publish: client=%s topic=%s", clientID, publish.Topic)
	}
	return nil
}

func (p *ACLPlugin) evaluator(ctx context.Context) (*acl.StaticEvaluator, error) {
	if p == nil || p.manager == nil {
		return nil, fmt.Errorf("acl manager is nil")
	}
	if err := p.manager.EnsureLoaded(ctx); err != nil {
		return nil, err
	}
	ev := p.manager.Current()
	if ev == nil {
		return nil, fmt.Errorf("acl evaluator is nil")
	}
	return ev, nil
}

func (p *ACLPlugin) username(clientID string) string {
	if clientID == "" {
		return ""
	}
	if v, ok := p.usernameByClient.Load(clientID); ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}
