package acl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type stubSource struct {
	load func(ctx context.Context) (*Ruleset, error)
}

func (s stubSource) Load(ctx context.Context) (*Ruleset, error) {
	if s.load == nil {
		return nil, nil
	}
	return s.load(ctx)
}

func TestLoader_NoSource(t *testing.T) {
	t.Parallel()

	l := NewLoader(LoaderConfig{})
	_, err := l.LoadOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoader_FileTakesPrecedenceOverFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "acl.yaml")

	rs := Ruleset{
		DefaultDeny: true,
		Rules: []Rule{
			{Identity: Identity{Username: "u1"}, Allow: RuleEntry{Pub: []string{"file/#"}}},
		},
	}
	b, err := yaml.Marshal(&rs)
	if err != nil {
		t.Fatalf("yaml marshal err=%v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write file err=%v", err)
	}

	l := NewLoader(LoaderConfig{
		FilePath: path,
		Fallback: stubSource{load: func(context.Context) (*Ruleset, error) {
			return nil, errors.New("fallback should not be used")
		}},
	})

	ev, err := l.LoadOnce(context.Background())
	if err != nil || ev == nil {
		t.Fatalf("LoadOnce err=%v ev=nil=%v", err, ev == nil)
	}

	ok, err := ev.AllowPublish(context.Background(), "u1", "", "file/a")
	if err != nil || !ok {
		t.Fatalf("expected allow from file rules; ok=%v err=%v", ok, err)
	}
}
