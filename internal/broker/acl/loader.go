package acl

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Loader loads rules from file (preferred when exists) otherwise from a fallback source,
// and caches the compiled evaluator in memory.
type Loader struct {
	filePath string
	fileSrc  Source
	fallback Source

	defaultDeny bool

	eval atomic.Value // *StaticEvaluator
}

type LoaderConfig struct {
	FilePath     string
	DefaultDeny  bool
	Fallback     Source
	FileSource   Source
	ReloadEvery  time.Duration
	DisableCache bool
}

func NewLoader(cfg LoaderConfig) *Loader {
	l := &Loader{
		filePath:    cfg.FilePath,
		defaultDeny: cfg.DefaultDeny,
		fallback:    cfg.Fallback,
		fileSrc:     cfg.FileSource,
	}
	return l
}

func (l *Loader) LoadOnce(ctx context.Context) (*StaticEvaluator, error) {
	rs, err := l.loadRules(ctx)
	if err != nil {
		return nil, err
	}
	ev := NewStaticEvaluator(*rs)
	l.eval.Store(ev)
	return ev, nil
}

func (l *Loader) Current() *StaticEvaluator {
	if v := l.eval.Load(); v != nil {
		if ev, ok := v.(*StaticEvaluator); ok {
			return ev
		}
	}
	return nil
}

func (l *Loader) loadRules(ctx context.Context) (*Ruleset, error) {
	// File takes precedence when exists.
	if l.filePath != "" && FileExists(l.filePath) {
		src := l.fileSrc
		if src == nil {
			src = &FileSource{Path: l.filePath}
		}
		rs, err := src.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("load acl from file failed: %w", err)
		}
		if rs != nil {
			if rs.DefaultDeny == false && l.defaultDeny {
				rs.DefaultDeny = true
			}
		}
		return rs, nil
	}

	if l.fallback == nil {
		return nil, fmt.Errorf("no acl source available")
	}
	rs, err := l.fallback.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load acl from keystore failed: %w", err)
	}
	if rs != nil {
		if rs.DefaultDeny == false && l.defaultDeny {
			rs.DefaultDeny = true
		}
	}
	return rs, nil
}
