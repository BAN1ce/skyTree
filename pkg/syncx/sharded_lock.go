package syncx

import (
	"hash/fnv"
	"sync"
)

const numLocks = 16

type ShardedLock struct {
	lockCount int
	locks     []sync.RWMutex
}

func NewShardedLock(lockCount int) *ShardedLock {
	return &ShardedLock{
		lockCount: lockCount,
		locks:     make([]sync.RWMutex, lockCount),
	}
}

func (s *ShardedLock) getLock(key string) *sync.RWMutex {
	hash := fnv.New32a()
	hash.Write([]byte(key))
	index := hash.Sum32() % uint32(s.lockCount)
	return &s.locks[index]
}

func (s *ShardedLock) Lock(key string) {
	s.getLock(key).Lock()
}

func (s *ShardedLock) Unlock(key string) {
	s.getLock(key).Unlock()
}

func (s *ShardedLock) RLock(key string) {
	s.getLock(key).RLock()
}
func (s *ShardedLock) RUnlock(key string) {
	s.getLock(key).RUnlock()
}
