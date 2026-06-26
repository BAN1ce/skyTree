package raftstore

import (
	"context"
	"testing"

	"github.com/BAN1ce/skyTree/pkg/cluster/dbpb"
	"github.com/lni/dragonboat/v3/statemachine"
)

type keyStoreClusterTestClient struct {
	readFn func(ctx context.Context, query interface{}) (interface{}, error)
}

func (c *keyStoreClusterTestClient) Write(context.Context, []byte) (statemachine.Result, error) {
	return statemachine.Result{}, nil
}

func (c *keyStoreClusterTestClient) Read(ctx context.Context, query interface{}) (interface{}, error) {
	if c.readFn == nil {
		return nil, nil
	}
	return c.readFn(ctx, query)
}

func (c *keyStoreClusterTestClient) GetNodeID() uint64 { return 1 }

func TestKeyStoreClusterReadPrefixKeyRejectsOddResult(t *testing.T) {
	s := NewKeyStoreCluster(&keyStoreClusterTestClient{
		readFn: func(context.Context, interface{}) (interface{}, error) {
			return &dbpb.Response{Result: [][]byte{
				[]byte("k1"), []byte("v1"), []byte("k2"),
			}}, nil
		},
	})

	got, err := s.ReadPrefixKey(context.Background(), []byte("p"))
	if err == nil {
		t.Fatal("ReadPrefixKey() expected error for odd result length, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("ReadPrefixKey() result = %v, want empty map on error", got)
	}
}

func TestKeyStoreClusterReadPrefixKeyParsesEvenResult(t *testing.T) {
	s := NewKeyStoreCluster(&keyStoreClusterTestClient{
		readFn: func(context.Context, interface{}) (interface{}, error) {
			return &dbpb.Response{Result: [][]byte{
				[]byte("k1"), []byte("v1"), []byte("k2"), []byte("v2"),
			}}, nil
		},
	})

	got, err := s.ReadPrefixKey(context.Background(), []byte("p"))
	if err != nil {
		t.Fatalf("ReadPrefixKey() error = %v", err)
	}
	if len(got) != 2 || got["k1"] != "v1" || got["k2"] != "v2" {
		t.Fatalf("ReadPrefixKey() result = %v, want map[k1:v1 k2:v2]", got)
	}
}

func TestKeyStoreClusterReadPrefixKeyNonResponseKeepsBehavior(t *testing.T) {
	s := NewKeyStoreCluster(&keyStoreClusterTestClient{
		readFn: func(context.Context, interface{}) (interface{}, error) {
			return struct{}{}, nil
		},
	})

	got, err := s.ReadPrefixKey(context.Background(), []byte("p"))
	if err != nil {
		t.Fatalf("ReadPrefixKey() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadPrefixKey() result = %v, want empty map", got)
	}
}

func TestKeyStoreClusterHGetAllRejectsOddResult(t *testing.T) {
	s := NewKeyStoreCluster(&keyStoreClusterTestClient{
		readFn: func(context.Context, interface{}) (interface{}, error) {
			return &dbpb.Response{Result: [][]byte{
				[]byte("k1"), []byte("v1"), []byte("k2"),
			}}, nil
		},
	})

	got, err := s.HGetAll(context.Background(), []byte("hash"))
	if err == nil {
		t.Fatal("HGetAll() expected error for odd result length, got nil")
	}
	if got != nil {
		t.Fatalf("HGetAll() result = %v, want nil on error", got)
	}
}

func TestKeyStoreClusterHPrefixRejectsOddResult(t *testing.T) {
	s := NewKeyStoreCluster(&keyStoreClusterTestClient{
		readFn: func(context.Context, interface{}) (interface{}, error) {
			return &dbpb.Response{Result: [][]byte{
				[]byte("k1"), []byte("v1"), []byte("k2"),
			}}, nil
		},
	})

	got, err := s.HPrefix(context.Background(), []byte("hash"), []byte("p"))
	if err == nil {
		t.Fatal("HPrefix() expected error for odd result length, got nil")
	}
	if got != nil {
		t.Fatalf("HPrefix() result = %v, want nil on error", got)
	}
}

func TestKeyStoreClusterHashReadsParseEvenResult(t *testing.T) {
	tests := []struct {
		name string
		call func(*KeyStoreCluster) (map[string]string, error)
	}{
		{
			name: "hgetall",
			call: func(s *KeyStoreCluster) (map[string]string, error) {
				return s.HGetAll(context.Background(), []byte("hash"))
			},
		},
		{
			name: "hprefix",
			call: func(s *KeyStoreCluster) (map[string]string, error) {
				return s.HPrefix(context.Background(), []byte("hash"), []byte("p"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewKeyStoreCluster(&keyStoreClusterTestClient{
				readFn: func(context.Context, interface{}) (interface{}, error) {
					return &dbpb.Response{Result: [][]byte{
						[]byte("k1"), []byte("v1"), []byte("k2"), []byte("v2"),
					}}, nil
				},
			})

			got, err := tt.call(s)
			if err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			if len(got) != 2 || got["k1"] != "v1" || got["k2"] != "v2" {
				t.Fatalf("%s result = %v, want map[k1:v1 k2:v2]", tt.name, got)
			}
		})
	}
}

func TestKeyStoreClusterHashReadsUnexpectedResponseType(t *testing.T) {
	tests := []struct {
		name string
		call func(*KeyStoreCluster) (map[string]string, error)
	}{
		{
			name: "hgetall",
			call: func(s *KeyStoreCluster) (map[string]string, error) {
				return s.HGetAll(context.Background(), []byte("hash"))
			},
		},
		{
			name: "hprefix",
			call: func(s *KeyStoreCluster) (map[string]string, error) {
				return s.HPrefix(context.Background(), []byte("hash"), []byte("p"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewKeyStoreCluster(&keyStoreClusterTestClient{
				readFn: func(context.Context, interface{}) (interface{}, error) {
					return struct{}{}, nil
				},
			})

			got, err := tt.call(s)
			if err == nil {
				t.Fatalf("%s expected error for unexpected response type, got nil", tt.name)
			}
			if got != nil {
				t.Fatalf("%s result = %v, want nil on error", tt.name, got)
			}
		})
	}
}
