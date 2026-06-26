package acl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"gopkg.in/yaml.v3"
)

var (
	// ErrFileModeActive indicates ACL is loaded from a local file and KeyStore changes won't take effect.
	ErrFileModeActive = errors.New("acl file mode is active")
	// ErrRulesetNotFound indicates no ruleset is present in KeyStore.
	ErrRulesetNotFound = errors.New("acl ruleset not found")
	// ErrInvalidIdentity indicates both username and client_id are empty.
	ErrInvalidIdentity = errors.New("invalid identity: username and client_id are both empty")
)

type ManagerConfig struct {
	Store       store.KVStore
	KeyStoreKey string
	FilePath    string
	DefaultDeny bool
}

// Manager manages ACL rules in KeyStore and provides a hot-reloadable evaluator.
// File mode takes precedence when the configured file exists.
type Manager struct {
	store       store.KVStore
	key         []byte
	filePath    string
	defaultDeny bool

	mu   sync.Mutex
	eval atomic.Value // *evalHolder
}

type evalHolder struct {
	ev *StaticEvaluator
}

func NewManager(cfg ManagerConfig) *Manager {
	m := &Manager{
		store:       cfg.Store,
		key:         []byte(cfg.KeyStoreKey),
		filePath:    cfg.FilePath,
		defaultDeny: cfg.DefaultDeny,
	}
	// Initialize so we can later store a nil evaluator via a non-nil holder.
	m.eval.Store(&evalHolder{ev: nil})
	return m
}

func (m *Manager) FileModeActive() bool {
	return m != nil && m.filePath != "" && FileExists(m.filePath)
}

func (m *Manager) Current() *StaticEvaluator {
	if m == nil {
		return nil
	}
	if v := m.eval.Load(); v != nil {
		if h, ok := v.(*evalHolder); ok {
			return h.ev
		}
	}
	return nil
}

func (m *Manager) EnsureLoaded(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("acl manager is nil")
	}
	if m.Current() != nil {
		return nil
	}
	return m.Reload(ctx)
}

// Reload reloads rules (file preferred when exists) and replaces the current evaluator.
func (m *Manager) Reload(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("acl manager is nil")
	}
	rs, err := m.loadRulesStrict(ctx)
	if err != nil {
		return err
	}
	m.applyDefaultDeny(rs)
	m.setCurrent(NewStaticEvaluator(*rs))
	return nil
}

func (m *Manager) GetRuleset(ctx context.Context) (*Ruleset, bool, error) {
	if m == nil {
		return nil, false, fmt.Errorf("acl manager is nil")
	}
	rs, found, err := m.loadRules(ctx)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	m.applyDefaultDeny(rs)
	return rs, true, nil
}

func (m *Manager) PutRuleset(ctx context.Context, rs Ruleset) error {
	if m == nil {
		return fmt.Errorf("acl manager is nil")
	}
	if m.FileModeActive() {
		return ErrFileModeActive
	}
	if err := validateRuleset(rs); err != nil {
		return err
	}
	m.applyDefaultDeny(&rs)

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.putRulesetLocked(ctx, &rs); err != nil {
		return err
	}
	m.setCurrent(NewStaticEvaluator(rs))
	return nil
}

func (m *Manager) DeleteRuleset(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("acl manager is nil")
	}
	if m.FileModeActive() {
		return ErrFileModeActive
	}
	if m.store == nil {
		return fmt.Errorf("keystore is nil")
	}
	if len(m.key) == 0 {
		return fmt.Errorf("acl keystore key is empty")
	}
	if err := m.store.DeleteKey(ctx, m.key); err != nil {
		return err
	}
	m.setCurrent(nil)
	return nil
}

func (m *Manager) GetRule(ctx context.Context, id Identity) (*Rule, bool, error) {
	if m == nil {
		return nil, false, fmt.Errorf("acl manager is nil")
	}
	if err := validateIdentity(id); err != nil {
		return nil, false, err
	}
	rs, found, err := m.loadRules(ctx)
	if err != nil {
		return nil, false, err
	}
	if !found || rs == nil {
		return nil, false, nil
	}
	for i := range rs.Rules {
		if sameIdentity(rs.Rules[i].Identity, id) {
			r := rs.Rules[i]
			return &r, true, nil
		}
	}
	return nil, false, nil
}

func (m *Manager) UpsertRule(ctx context.Context, rule Rule) error {
	if m == nil {
		return fmt.Errorf("acl manager is nil")
	}
	if m.FileModeActive() {
		return ErrFileModeActive
	}
	if err := validateIdentity(rule.Identity); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	rs, found, err := m.loadRules(ctx)
	if err != nil {
		return err
	}
	if !found || rs == nil {
		rs = &Ruleset{DefaultDeny: m.defaultDeny}
	}
	m.applyDefaultDeny(rs)

	updated := false
	for i := range rs.Rules {
		if sameIdentity(rs.Rules[i].Identity, rule.Identity) {
			rs.Rules[i] = rule
			updated = true
			break
		}
	}
	if !updated {
		rs.Rules = append(rs.Rules, rule)
	}

	if err := m.putRulesetLocked(ctx, rs); err != nil {
		return err
	}
	m.setCurrent(NewStaticEvaluator(*rs))
	return nil
}

func (m *Manager) DeleteRule(ctx context.Context, id Identity) (bool, error) {
	if m == nil {
		return false, fmt.Errorf("acl manager is nil")
	}
	if m.FileModeActive() {
		return false, ErrFileModeActive
	}
	if err := validateIdentity(id); err != nil {
		return false, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	rs, found, err := m.loadRules(ctx)
	if err != nil {
		return false, err
	}
	if !found || rs == nil {
		return false, nil
	}

	out := rs.Rules[:0]
	removed := false
	for _, r := range rs.Rules {
		if sameIdentity(r.Identity, id) {
			removed = true
			continue
		}
		out = append(out, r)
	}
	rs.Rules = out
	m.applyDefaultDeny(rs)

	if !removed {
		return false, nil
	}
	if err := m.putRulesetLocked(ctx, rs); err != nil {
		return false, err
	}
	m.setCurrent(NewStaticEvaluator(*rs))
	return true, nil
}

func (m *Manager) setCurrent(ev *StaticEvaluator) {
	m.eval.Store(&evalHolder{ev: ev})
}

func (m *Manager) putRulesetLocked(ctx context.Context, rs *Ruleset) error {
	if m.store == nil {
		return fmt.Errorf("keystore is nil")
	}
	if len(m.key) == 0 {
		return fmt.Errorf("acl keystore key is empty")
	}
	b, err := yaml.Marshal(rs)
	if err != nil {
		return err
	}
	return m.store.PutKey(ctx, m.key, b)
}

func (m *Manager) loadRulesStrict(ctx context.Context) (*Ruleset, error) {
	rs, found, err := m.loadRules(ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrRulesetNotFound
	}
	return rs, nil
}

func (m *Manager) loadRules(ctx context.Context) (*Ruleset, bool, error) {
	// File takes precedence when exists.
	if m.FileModeActive() {
		src := &FileSource{Path: m.filePath}
		rs, err := src.Load(ctx)
		if err != nil {
			return nil, false, err
		}
		return rs, true, nil
	}

	if m.store == nil {
		return nil, false, fmt.Errorf("keystore is nil")
	}
	if len(m.key) == 0 {
		return nil, false, fmt.Errorf("acl keystore key is empty")
	}
	b, ok, err := m.store.ReadKey(ctx, m.key)
	if err != nil {
		return nil, false, err
	}
	if !ok || len(b) == 0 {
		return nil, false, nil
	}
	var rs Ruleset
	if err := yaml.Unmarshal(b, &rs); err != nil {
		return nil, false, err
	}
	return &rs, true, nil
}

func (m *Manager) applyDefaultDeny(rs *Ruleset) {
	if rs == nil {
		return
	}
	if rs.DefaultDeny == false && m.defaultDeny {
		rs.DefaultDeny = true
	}
}

func validateRuleset(rs Ruleset) error {
	for _, r := range rs.Rules {
		if err := validateIdentity(r.Identity); err != nil {
			return err
		}
	}
	return nil
}

func validateIdentity(id Identity) error {
	if id.Username == "" && id.ClientID == "" {
		return ErrInvalidIdentity
	}
	return nil
}

func sameIdentity(a, b Identity) bool {
	return a.Username == b.Username && a.ClientID == b.ClientID
}
