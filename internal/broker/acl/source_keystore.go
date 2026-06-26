package acl

import (
	"context"
	"fmt"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"gopkg.in/yaml.v3"
)

type KeyStoreSource struct {
	Store store.KVReader
	Key   []byte
}

func (s *KeyStoreSource) Load(ctx context.Context) (*Ruleset, error) {
	if s == nil || s.Store == nil {
		return nil, fmt.Errorf("keystore is nil")
	}
	if len(s.Key) == 0 {
		return nil, fmt.Errorf("acl keystore key is empty")
	}
	b, ok, err := s.Store.ReadKey(ctx, s.Key)
	if err != nil {
		return nil, err
	}
	if !ok || len(b) == 0 {
		return nil, fmt.Errorf("acl rules not found in keystore")
	}
	var rs Ruleset
	if err := yaml.Unmarshal(b, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}
