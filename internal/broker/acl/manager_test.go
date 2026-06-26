package acl

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"gopkg.in/yaml.v3"
)

type fakeKeyStore struct {
	mu sync.Mutex
	kv map[string][]byte
}

var _ store.KeyStore = (*fakeKeyStore)(nil)

func newFakeKeyStore() *fakeKeyStore {
	return &fakeKeyStore{kv: make(map[string][]byte)}
}

func (s *fakeKeyStore) PutKey(_ context.Context, key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[string(key)] = append([]byte(nil), value...)
	return nil
}

func (s *fakeKeyStore) ReadKey(_ context.Context, key []byte) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.kv[string(key)]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), v...), true, nil
}

func (s *fakeKeyStore) DeleteKey(_ context.Context, key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.kv, string(key))
	return nil
}

func (s *fakeKeyStore) DeletePrefixKey(_ context.Context, prefix []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.kv {
		if bytes.HasPrefix([]byte(k), prefix) {
			delete(s.kv, k)
		}
	}
	return nil
}

func (s *fakeKeyStore) Close() error { return nil }

func (s *fakeKeyStore) SetExpired(_ context.Context, _ []byte, _ time.Duration) error { return nil }

// HashStore methods (not used by acl tests, but required by interface).
func (s *fakeKeyStore) HSet(ctx context.Context, key []byte, field [][]byte) error {
	for i := 0; i < len(field); i += 2 {
		if i+1 >= len(field) {
			return nil
		}
		k := append(append([]byte(nil), key...), field[i]...)
		if err := s.PutKey(ctx, k, field[i+1]); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakeKeyStore) HGet(ctx context.Context, key, field []byte) ([]byte, bool, error) {
	k := append(append([]byte(nil), key...), field...)
	return s.ReadKey(ctx, k)
}

func (s *fakeKeyStore) HDel(ctx context.Context, key []byte, field [][]byte) error {
	for _, f := range field {
		k := append(append([]byte(nil), key...), f...)
		if err := s.DeleteKey(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakeKeyStore) HGetAll(_ context.Context, key []byte) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string)
	for k, v := range s.kv {
		if bytes.HasPrefix([]byte(k), key) {
			out[k] = string(v)
		}
	}
	return out, nil
}

func (s *fakeKeyStore) HPrefix(_ context.Context, key []byte, prefix []byte) (map[string]string, error) {
	return s.HGetAll(context.Background(), append(key, prefix...))
}

func (s *fakeKeyStore) DeleteHash(ctx context.Context, key []byte) error {
	return s.DeletePrefixKey(ctx, key)
}

func TestManager_EnsureLoaded_NoRuleset(t *testing.T) {
	t.Parallel()

	ks := newFakeKeyStore()
	m := NewManager(ManagerConfig{
		Store:       ks,
		KeyStoreKey: "acl",
		DefaultDeny: true,
	})

	err := m.EnsureLoaded(context.Background())
	if !errors.Is(err, ErrRulesetNotFound) {
		t.Fatalf("EnsureLoaded err=%v want ErrRulesetNotFound", err)
	}
}

func TestManager_PutGetRuleset_AndDefaultDenyApplied(t *testing.T) {
	t.Parallel()

	ks := newFakeKeyStore()
	m := NewManager(ManagerConfig{
		Store:       ks,
		KeyStoreKey: "acl",
		DefaultDeny: true,
	})

	in := Ruleset{
		DefaultDeny: false,
		Rules: []Rule{
			{Identity: Identity{Username: "u1"}, Allow: RuleEntry{Pub: []string{"a/#"}}},
		},
	}

	if err := m.PutRuleset(context.Background(), in); err != nil {
		t.Fatalf("PutRuleset err=%v", err)
	}

	// Stored value should be a valid ruleset YAML.
	b, ok, err := ks.ReadKey(context.Background(), []byte("acl"))
	if err != nil || !ok || len(b) == 0 {
		t.Fatalf("keystore ReadKey ok=%v err=%v len=%d", ok, err, len(b))
	}
	var stored Ruleset
	if err := yaml.Unmarshal(b, &stored); err != nil {
		t.Fatalf("unmarshal stored ruleset err=%v", err)
	}
	if stored.DefaultDeny != true {
		t.Fatalf("stored.DefaultDeny=%v want=true", stored.DefaultDeny)
	}

	// Evaluator should reflect default deny (no matching identity -> deny).
	ev := m.Current()
	if ev == nil {
		t.Fatalf("Current evaluator is nil")
	}
	if ok, err := ev.AllowPublish(context.Background(), "someone-else", "", "a/b"); err != nil || ok {
		t.Fatalf("expected default deny; ok=%v err=%v", ok, err)
	}

	rs, found, err := m.GetRuleset(context.Background())
	if err != nil || !found || rs == nil {
		t.Fatalf("GetRuleset found=%v err=%v rs=nil=%v", found, err, rs == nil)
	}
	if rs.DefaultDeny != true {
		t.Fatalf("GetRuleset.DefaultDeny=%v want=true", rs.DefaultDeny)
	}
}

func TestManager_UpsertRule_UpdateExistingAndDeleteRule(t *testing.T) {
	t.Parallel()

	ks := newFakeKeyStore()
	m := NewManager(ManagerConfig{
		Store:       ks,
		KeyStoreKey: "acl",
	})

	r1 := Rule{
		Identity: Identity{Username: "u1", ClientID: "c1"},
		Allow:    RuleEntry{Pub: []string{"a/+"}},
	}
	if err := m.UpsertRule(context.Background(), r1); err != nil {
		t.Fatalf("UpsertRule err=%v", err)
	}

	r2 := Rule{
		Identity: Identity{Username: "u1", ClientID: "c1"},
		Allow:    RuleEntry{Pub: []string{"b/+"}},
	}
	if err := m.UpsertRule(context.Background(), r2); err != nil {
		t.Fatalf("UpsertRule(update) err=%v", err)
	}

	got, ok, err := m.GetRule(context.Background(), Identity{Username: "u1", ClientID: "c1"})
	if err != nil || !ok || got == nil {
		t.Fatalf("GetRule ok=%v err=%v got=nil=%v", ok, err, got == nil)
	}
	if len(got.Allow.Pub) != 1 || got.Allow.Pub[0] != "b/+" {
		t.Fatalf("rule not updated: %+v", *got)
	}

	rs, found, err := m.GetRuleset(context.Background())
	if err != nil || !found || rs == nil {
		t.Fatalf("GetRuleset found=%v err=%v rs=nil=%v", found, err, rs == nil)
	}
	if len(rs.Rules) != 1 {
		t.Fatalf("expected 1 rule after upsert update; got=%d", len(rs.Rules))
	}

	removed, err := m.DeleteRule(context.Background(), Identity{Username: "u1", ClientID: "c1"})
	if err != nil || !removed {
		t.Fatalf("DeleteRule removed=%v err=%v", removed, err)
	}
	_, ok, err = m.GetRule(context.Background(), Identity{Username: "u1", ClientID: "c1"})
	if err != nil || ok {
		t.Fatalf("rule should be deleted; ok=%v err=%v", ok, err)
	}
}

func TestManager_InvalidIdentityRejected(t *testing.T) {
	t.Parallel()

	m := NewManager(ManagerConfig{
		Store:       newFakeKeyStore(),
		KeyStoreKey: "acl",
	})

	if err := m.UpsertRule(context.Background(), Rule{}); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("UpsertRule err=%v want ErrInvalidIdentity", err)
	}
	if _, _, err := m.GetRule(context.Background(), Identity{}); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("GetRule err=%v want ErrInvalidIdentity", err)
	}
}

func TestManager_FileModePrecedence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "acl.yaml")

	fileRules := Ruleset{
		DefaultDeny: true,
		Rules: []Rule{
			{Identity: Identity{Username: "u_file"}, Allow: RuleEntry{Pub: []string{"file/#"}}},
		},
	}
	b, err := yaml.Marshal(&fileRules)
	if err != nil {
		t.Fatalf("yaml marshal err=%v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write file err=%v", err)
	}

	ks := newFakeKeyStore()
	_ = ks.PutKey(context.Background(), []byte("acl"), []byte("default_deny: false\nrules: []\n"))

	m := NewManager(ManagerConfig{
		Store:       ks,
		KeyStoreKey: "acl",
		FilePath:    path,
	})

	if !m.FileModeActive() {
		t.Fatalf("FileModeActive should be true")
	}
	if err := m.PutRuleset(context.Background(), Ruleset{DefaultDeny: false}); !errors.Is(err, ErrFileModeActive) {
		t.Fatalf("PutRuleset err=%v want ErrFileModeActive", err)
	}

	rs, found, err := m.GetRuleset(context.Background())
	if err != nil || !found || rs == nil {
		t.Fatalf("GetRuleset found=%v err=%v rs=nil=%v", found, err, rs == nil)
	}
	if len(rs.Rules) != 1 || rs.Rules[0].Identity.Username != "u_file" {
		t.Fatalf("expected rules from file, got=%+v", rs)
	}
}
