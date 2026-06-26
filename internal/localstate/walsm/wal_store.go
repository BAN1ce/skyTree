package walsm

import (
	"errors"
	"os"

	"github.com/BAN1ce/skyTree/logger"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
)

var errEmptyWAL = errors.New("wal_sm: empty wal")

type WALStore struct {
	dir string
	w   *wal.WAL
	hs  raftpb.HardState
}

func OpenOrCreateWAL(dir string, snap walpb.Snapshot) (*WALStore, []raftpb.Entry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}

	if !wal.Exist(dir) {
		w, err := wal.Create(zap.NewNop(), dir, []byte("skytree-local-wal-v1"))
		if err != nil {
			return nil, nil, err
		}
		s := &WALStore{dir: dir, w: w}
		// etcd WAL requires the first raft entry index to be 0. Reserve index=0 as a no-op record.
		if err := s.Save(0, []raftpb.Entry{{Index: 0, Term: 1, Type: raftpb.EntryNormal}}); err != nil {
			_ = s.Close()
			return nil, nil, err
		}
		return s, nil, nil
	}

	return openExistingWAL(dir, snap)
}

func openExistingWAL(dir string, snap walpb.Snapshot) (*WALStore, []raftpb.Entry, error) {
	w, hs, ents, err := openReadAll(dir, snap)
	if err == nil {
		return &WALStore{dir: dir, w: w, hs: hs}, ents, nil
	}

	// etcd WAL can return ErrSnapshotMismatch/ErrSnapshotNotFound while still returning all records.
	// For a local single-node WAL, we prefer to proceed with the returned hardstate/entries and
	// let the upper layer filter entries by applied index (snapshot index).
	if errors.Is(err, wal.ErrSnapshotNotFound) || errors.Is(err, wal.ErrSnapshotMismatch) {
		logger.Logger.Warn().Err(err).Str("wal_dir", dir).Msg("wal snapshot mismatch, proceeding with full WAL scan")
		return &WALStore{dir: dir, w: w, hs: hs}, ents, nil
	}

	// Unexpected EOF: attempt repair once.
	if wal.Repair(zap.NewNop(), dir) {
		logger.Logger.Warn().Str("wal_dir", dir).Msg("wal repaired, retry open")
		w3, hs3, ents3, err3 := openReadAll(dir, snap)
		if err3 == nil {
			return &WALStore{dir: dir, w: w3, hs: hs3}, ents3, nil
		}
		err = err3
	}

	return nil, nil, err
}

func openReadAll(dir string, snap walpb.Snapshot) (*wal.WAL, raftpb.HardState, []raftpb.Entry, error) {
	w, err := wal.Open(zap.NewNop(), dir, snap)
	if err != nil {
		return nil, raftpb.HardState{}, nil, err
	}
	_, hs, ents, err := w.ReadAll()
	if err != nil && !errors.Is(err, wal.ErrSnapshotNotFound) && !errors.Is(err, wal.ErrSnapshotMismatch) {
		_ = w.Close()
		return nil, raftpb.HardState{}, nil, err
	}
	return w, hs, ents, err
}

func (s *WALStore) Save(commitIndex uint64, ents []raftpb.Entry) error {
	if s.w == nil {
		return errEmptyWAL
	}
	hs := s.hs
	hs.Term = 1
	hs.Vote = 1
	hs.Commit = commitIndex
	if err := s.w.Save(hs, ents); err != nil {
		return err
	}
	s.hs = hs
	return s.w.Sync()
}

func (s *WALStore) SaveSnapshot(meta SnapshotMeta) error {
	if s.w == nil {
		return errEmptyWAL
	}
	if err := s.w.SaveSnapshot(walpb.Snapshot{Index: meta.Index, Term: meta.Term}); err != nil {
		return err
	}
	return s.w.Sync()
}

func (s *WALStore) ReleaseLockTo(index uint64) error {
	if s.w == nil {
		return errEmptyWAL
	}
	return s.w.ReleaseLockTo(index)
}

func (s *WALStore) Close() error {
	if s.w == nil {
		return nil
	}
	return s.w.Close()
}
