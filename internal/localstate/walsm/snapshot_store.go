package walsm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// SnapshotMeta is the minimal metadata we need to align snapshot files with WAL snapshot records.
type SnapshotMeta struct {
	Index uint64
	Term  uint64
}

type SnapshotStore struct {
	dir string
}

func NewSnapshotStore(dir string) *SnapshotStore {
	return &SnapshotStore{dir: dir}
}

func (s *SnapshotStore) EnsureDir() error {
	return os.MkdirAll(s.dir, 0o755)
}

func (s *SnapshotStore) Save(meta SnapshotMeta, data []byte) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, fmt.Sprintf(".snapshot-%d-%d.tmp", meta.Index, meta.Term))
	dst := filepath.Join(s.dir, snapshotFileName(meta))

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return s.cleanupKeepOnly(dst)
}

func (s *SnapshotStore) LoadLatest() (SnapshotMeta, []byte, bool, error) {
	if err := s.EnsureDir(); err != nil {
		return SnapshotMeta{}, nil, false, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return SnapshotMeta{}, nil, false, err
	}
	candidates := make([]snapshotCandidate, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		meta, ok := parseSnapshotFileName(e.Name())
		if !ok {
			continue
		}
		candidates = append(candidates, snapshotCandidate{meta: meta, name: e.Name()})
	}
	if len(candidates) == 0 {
		return SnapshotMeta{}, nil, false, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].meta.Index != candidates[j].meta.Index {
			return candidates[i].meta.Index > candidates[j].meta.Index
		}
		return candidates[i].meta.Term > candidates[j].meta.Term
	})
	best := candidates[0]
	path := filepath.Join(s.dir, best.name)
	b, err := os.ReadFile(path)
	if err != nil {
		return SnapshotMeta{}, nil, false, err
	}
	return best.meta, b, true, nil
}

type snapshotCandidate struct {
	meta SnapshotMeta
	name string
}

func snapshotFileName(meta SnapshotMeta) string {
	return fmt.Sprintf("snapshot-%d-%d.bin", meta.Index, meta.Term)
}

func parseSnapshotFileName(name string) (SnapshotMeta, bool) {
	// snapshot-<index>-<term>.bin
	if !strings.HasPrefix(name, "snapshot-") || !strings.HasSuffix(name, ".bin") {
		return SnapshotMeta{}, false
	}
	base := strings.TrimSuffix(strings.TrimPrefix(name, "snapshot-"), ".bin")
	parts := strings.Split(base, "-")
	if len(parts) != 2 {
		return SnapshotMeta{}, false
	}
	idx, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return SnapshotMeta{}, false
	}
	term, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return SnapshotMeta{}, false
	}
	return SnapshotMeta{Index: idx, Term: term}, true
}

func (s *SnapshotStore) cleanupKeepOnly(keep string) error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		if path == keep {
			continue
		}
		_ = os.Remove(path)
	}
	return nil
}
