package walsm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/lni/dragonboat/v3/statemachine"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

const (
	walEntryMagic       = "skytree-wal-sm-v1"
	walEntryTypePending = byte(1)
	walEntryTypeCommit  = byte(2)
)

type smAdapter interface {
	Update([]byte) (statemachine.Result, error)
	Lookup(interface{}) (interface{}, error)
	SaveSnapshot(io.Writer, statemachine.ISnapshotFileCollection, <-chan struct{}) error
	RecoverFromSnapshot(io.Reader, []statemachine.SnapshotFile, <-chan struct{}) error
}

type updateValidator interface {
	ValidateUpdate([]byte) error
}

type walBackend interface {
	Save(uint64, []raftpb.Entry) error
	SaveSnapshot(SnapshotMeta) error
	ReleaseLockTo(uint64) error
	Close() error
}

type Engine struct {
	name string

	mu sync.RWMutex
	sm smAdapter

	wal  walBackend
	snap *SnapshotStore

	appliedIndex uint64
	nextIndex    uint64

	snapshotEntries  uint64
	snapshotInterval time.Duration
	sinceSnapshot    uint64

	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
	hasTicker bool
}

type Options struct {
	Name             string
	BaseDir          string
	SnapshotEntries  uint64
	SnapshotInterval time.Duration
}

func NewEngine(sm smAdapter, opt Options) (*Engine, error) {
	if sm == nil {
		return nil, fmt.Errorf("wal_sm: nil state machine")
	}
	if opt.BaseDir == "" {
		return nil, fmt.Errorf("wal_sm: empty base dir")
	}

	e := &Engine{
		name:             opt.Name,
		sm:               sm,
		snapshotEntries:  opt.SnapshotEntries,
		snapshotInterval: opt.SnapshotInterval,
		stopCh:           make(chan struct{}),
		doneCh:           make(chan struct{}),
	}

	if err := e.init(opt.BaseDir); err != nil {
		_ = e.Close()
		return nil, err
	}
	e.startTicker()
	return e, nil
}

func (e *Engine) init(baseDir string) error {
	walDir := filepath.Join(baseDir, "wal")
	snapDir := filepath.Join(baseDir, "snap")

	e.snap = NewSnapshotStore(snapDir)

	snapMeta, snapData, ok, err := e.snap.LoadLatest()
	if err != nil {
		return err
	}
	hasSnapshot := ok
	replayedAny := ok
	if ok {
		if err := e.restoreFromSnapshotBytes(snapData); err != nil {
			return err
		}
		e.appliedIndex = snapMeta.Index
	}

	walSnap := walpb.Snapshot{}
	if ok {
		walSnap.Index = snapMeta.Index
		walSnap.Term = snapMeta.Term
	}
	w, ents, err := OpenOrCreateWAL(walDir, walSnap)
	if err != nil {
		return err
	}
	e.wal = w

	replayedAny, lastApplied, lastWALIndex, err := e.replayWALEntries(ents, hasSnapshot)
	if err != nil {
		return err
	}
	e.appliedIndex = maxU64(e.appliedIndex, lastApplied)
	if replayedAny {
		e.nextIndex = maxU64(e.appliedIndex+1, lastWALIndex+1)
	} else {
		// Fresh start: index 0 is reserved as a no-op record, so real writes start from 1.
		e.nextIndex = maxU64(1, lastWALIndex+1)
	}
	return nil
}

func (e *Engine) replayWALEntries(ents []raftpb.Entry, hasSnapshot bool) (bool, uint64, uint64, error) {
	committed := committedPendingEntries(ents)
	replayedAny := hasSnapshot
	lastApplied := e.appliedIndex
	var lastWALIndex uint64

	for _, ent := range ents {
		lastWALIndex = maxU64(lastWALIndex, ent.Index)
		if shouldSkipReplayEntry(ent, hasSnapshot, e.appliedIndex) {
			continue
		}

		entryType, _, payload, wrapped := decodeWALEntry(ent.Data)
		switch {
		case !wrapped:
			if _, err := e.sm.Update(ent.Data); err != nil {
				return false, 0, 0, err
			}
			replayedAny = true
			lastApplied = ent.Index
		case entryType == walEntryTypePending:
			if _, ok := committed[ent.Index]; !ok {
				logger.Logger.Warn().
					Str("name", e.name).
					Uint64("index", ent.Index).
					Msg("skip uncommitted local WAL entry")
				continue
			}
			if _, err := e.sm.Update(payload); err != nil {
				return false, 0, 0, err
			}
			replayedAny = true
			lastApplied = ent.Index
		case entryType == walEntryTypeCommit:
			continue
		}
	}

	return replayedAny, lastApplied, lastWALIndex, nil
}

func committedPendingEntries(ents []raftpb.Entry) map[uint64]struct{} {
	committed := make(map[uint64]struct{})
	for _, ent := range ents {
		entryType, pendingIndex, _, wrapped := decodeWALEntry(ent.Data)
		if !wrapped || entryType != walEntryTypeCommit {
			continue
		}
		committed[pendingIndex] = struct{}{}
	}
	return committed
}

func shouldSkipReplayEntry(ent raftpb.Entry, hasSnapshot bool, appliedIndex uint64) bool {
	if ent.Type != raftpb.EntryNormal {
		return true
	}
	// Index 0 is reserved as a no-op record to satisfy etcd WAL requirements.
	if len(ent.Data) == 0 {
		return true
	}
	// Only skip compacted entries when we actually restored a snapshot.
	return hasSnapshot && ent.Index <= appliedIndex
}

func (e *Engine) startTicker() {
	if e.snapshotInterval <= 0 {
		return
	}
	e.hasTicker = true
	ticker := time.NewTicker(e.snapshotInterval)
	go func() {
		defer close(e.doneCh)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = e.maybeSnapshotByTimer()
			case <-e.stopCh:
				return
			}
		}
	}()
}

func (e *Engine) maybeSnapshotByTimer() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sinceSnapshot == 0 {
		return nil
	}
	return e.takeSnapshotLocked()
}

func (e *Engine) Write(updateBytes []byte) (statemachine.Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.validateUpdate(updateBytes); err != nil {
		return statemachine.Result{}, err
	}

	idx := e.nextIndex
	ent := raftpb.Entry{
		Index: idx,
		Term:  1,
		Type:  raftpb.EntryNormal,
		Data:  encodePendingWALEntry(idx, updateBytes),
	}
	if err := e.wal.Save(idx, []raftpb.Entry{ent}); err != nil {
		return statemachine.Result{}, err
	}
	e.nextIndex = idx + 1

	commitIdx := e.nextIndex
	commitEnt := raftpb.Entry{
		Index: commitIdx,
		Term:  1,
		Type:  raftpb.EntryNormal,
		Data:  encodeCommitWALEntry(idx),
	}
	if err := e.wal.Save(commitIdx, []raftpb.Entry{commitEnt}); err != nil {
		e.nextIndex = commitIdx + 1
		return statemachine.Result{}, err
	}
	e.nextIndex = commitIdx + 1

	res, err := e.sm.Update(updateBytes)
	if err != nil {
		return res, err
	}

	e.appliedIndex = idx
	e.sinceSnapshot++

	if e.snapshotEntries > 0 && e.sinceSnapshot >= e.snapshotEntries {
		if err := e.takeSnapshotLocked(); err != nil {
			logger.Logger.Error().Err(err).Str("name", e.name).Msg("snapshot failed after entry threshold")
		}
	}
	return res, nil
}

func (e *Engine) validateUpdate(updateBytes []byte) error {
	validator, ok := e.sm.(updateValidator)
	if !ok {
		return nil
	}
	return validator.ValidateUpdate(updateBytes)
}

func encodePendingWALEntry(index uint64, payload []byte) []byte {
	headerSize := len(walEntryMagic) + 1 + 8
	out := make([]byte, headerSize+len(payload))
	copy(out, walEntryMagic)
	out[len(walEntryMagic)] = walEntryTypePending
	binary.BigEndian.PutUint64(out[len(walEntryMagic)+1:headerSize], index)
	copy(out[headerSize:], payload)
	return out
}

func encodeCommitWALEntry(pendingIndex uint64) []byte {
	headerSize := len(walEntryMagic) + 1 + 8
	out := make([]byte, headerSize)
	copy(out, walEntryMagic)
	out[len(walEntryMagic)] = walEntryTypeCommit
	binary.BigEndian.PutUint64(out[len(walEntryMagic)+1:headerSize], pendingIndex)
	return out
}

func decodeWALEntry(data []byte) (byte, uint64, []byte, bool) {
	headerSize := len(walEntryMagic) + 1 + 8
	if len(data) < headerSize || !bytes.HasPrefix(data, []byte(walEntryMagic)) {
		return 0, 0, nil, false
	}
	entryType := data[len(walEntryMagic)]
	index := binary.BigEndian.Uint64(data[len(walEntryMagic)+1 : headerSize])
	return entryType, index, data[headerSize:], true
}

func (e *Engine) Read(req interface{}) (interface{}, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sm.Lookup(req)
}

func (e *Engine) Close() error {
	e.stopOnce.Do(func() { close(e.stopCh) })

	if e.hasTicker {
		<-e.doneCh
	} else {
		// If ticker wasn't started, doneCh is never closed; close it here.
		select {
		case <-e.doneCh:
		default:
			close(e.doneCh)
		}
	}

	if e.wal != nil {
		return e.wal.Close()
	}
	return nil
}

func (e *Engine) takeSnapshotLocked() error {
	// etcd WAL uses an implicit empty snapshot at index=0/term=0. Persisting a snapshot at index 0
	// can cause snapshot mismatch on restart and may leave WAL in read mode. Only snapshot after
	// at least one real entry beyond index 0 has been applied.
	if e.appliedIndex == 0 {
		return nil
	}
	data, err := e.saveSnapshotBytesLocked()
	if err != nil {
		return err
	}
	meta := SnapshotMeta{Index: e.appliedIndex, Term: 1}
	if err := e.snap.Save(meta, data); err != nil {
		return err
	}
	if err := e.wal.SaveSnapshot(meta); err != nil {
		return err
	}
	if err := e.wal.ReleaseLockTo(meta.Index); err != nil {
		return err
	}
	e.sinceSnapshot = 0
	return nil
}

func (e *Engine) saveSnapshotBytesLocked() ([]byte, error) {
	var buf bytes.Buffer
	stop := make(chan struct{})
	defer close(stop)
	if err := e.sm.SaveSnapshot(&buf, nil, stop); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *Engine) restoreFromSnapshotBytes(b []byte) error {
	stop := make(chan struct{})
	defer close(stop)
	return e.sm.RecoverFromSnapshot(bytes.NewReader(b), nil, stop)
}

func maxU64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
