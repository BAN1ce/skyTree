package scheduler

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testTask struct {
	key        string
	expireTime int64
}

func (t *testTask) GetKey() string {
	return t.key
}

func (t *testTask) GetExpireTime() int64 {
	return t.expireTime
}

func TestPassiveScheduler_Add(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	task := &testTask{
		key:        "task1",
		expireTime: time.Now().Add(5 * time.Second).UnixMicro(),
	}

	tw.Add(task)

	if tw.Size() != 1 {
		t.Errorf("Expected size 1, got %d", tw.Size())
	}
}

func TestPassiveScheduler_Remove(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	task := &testTask{
		key:        "task1",
		expireTime: time.Now().Add(5 * time.Second).UnixMicro(),
	}

	tw.Add(task)

	if !tw.Remove("task1") {
		t.Error("Expected Remove to return true")
	}

	if tw.Size() != 0 {
		t.Errorf("Expected size 0, got %d", tw.Size())
	}

	if tw.Remove("nonexistent") {
		t.Error("Expected Remove to return false for nonexistent task")
	}
}

func TestPassiveScheduler_GetExpired(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	now := time.Now()

	expiredTask := &testTask{
		key:        "expired",
		expireTime: now.Add(-1 * time.Second).UnixMicro(),
	}
	tw.Add(expiredTask)

	futureTask := &testTask{
		key:        "future",
		expireTime: now.Add(10 * time.Second).UnixMicro(),
	}
	tw.Add(futureTask)

	expired := tw.GetExpired(now.UnixMicro())

	if len(expired) != 1 {
		t.Errorf("Expected 1 expired task, got %d", len(expired))
	}

	if expired[0].GetKey() != "expired" {
		t.Errorf("Expected expired task key 'expired', got '%s'", expired[0].GetKey())
	}
}

func TestPassiveScheduler_Advance(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	initialPos := tw.CurrentPosition()
	tw.Advance()

	if tw.CurrentPosition() != initialPos+1 {
		t.Errorf("Expected position %d, got %d", initialPos+1, tw.CurrentPosition())
	}

	for i := 0; i < 10; i++ {
		tw.Advance()
	}

	want := initialPos + 11
	if tw.CurrentPosition() != want {
		t.Errorf("Expected position %d after heap scheduler advances, got %d", want, tw.CurrentPosition())
	}
}

func TestPassiveScheduler_ExpiredTaskIsImmediatelyDue(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	expiredTask := &testTask{
		key:        "expired",
		expireTime: time.Now().Add(-1 * time.Second).UnixMicro(),
	}

	tw.Add(expiredTask)

	expired := tw.GetExpired(time.Now().UnixMicro())

	if len(expired) != 1 {
		t.Errorf("Expected 1 expired task, got %d", len(expired))
	}
}

func TestPassiveScheduler_LongDelayTaskExpiresAtScheduledTime(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)
	now := time.Now()

	futureTask := &testTask{
		key:        "long-delay",
		expireTime: now.Add(25 * time.Second).UnixMicro(),
	}

	tw.Add(futureTask)

	if expired := tw.GetExpired(now.Add(24 * time.Second).UnixMicro()); len(expired) != 0 {
		t.Fatalf("Expected long-delay task to stay pending before schedule, got %d expired tasks", len(expired))
	}

	expired := tw.GetExpired(now.Add(25 * time.Second).UnixMicro())
	if len(expired) != 1 || expired[0].GetKey() != "long-delay" {
		t.Fatalf("Expected long-delay task to expire at scheduled time, got %+v", expired)
	}
}

func TestPassiveScheduler_DuplicateAdd(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	task1 := &testTask{
		key:        "task1",
		expireTime: time.Now().Add(5 * time.Second).UnixMicro(),
	}

	task2 := &testTask{
		key:        "task1",
		expireTime: time.Now().Add(10 * time.Second).UnixMicro(),
	}

	tw.Add(task1)
	if tw.Size() != 1 {
		t.Errorf("Expected size 1, got %d", tw.Size())
	}

	tw.Add(task2)
	if tw.Size() != 1 {
		t.Errorf("Expected size 1 after duplicate add, got %d", tw.Size())
	}
}

func TestPassiveScheduler_EmptyKey(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	task := &testTask{
		key:        "",
		expireTime: time.Now().Add(5 * time.Second).UnixMicro(),
	}

	tw.Add(task)

	if tw.Size() != 0 {
		t.Errorf("Expected size 0 for empty key, got %d", tw.Size())
	}
}

func TestPassiveScheduler_ConcurrentAdd(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	var wg sync.WaitGroup
	numTasks := 1000

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task := &testTask{
				key:        "task" + strconv.Itoa(id),
				expireTime: time.Now().Add(time.Duration(id) * time.Second).UnixMicro(),
			}
			tw.Add(task)
		}(i)
	}

	wg.Wait()

	if tw.Size() != numTasks {
		t.Errorf("Expected size %d, got %d", numTasks, tw.Size())
	}
}

func TestPassiveScheduler_ConcurrentRemove(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	numTasks := 100
	for i := 0; i < numTasks; i++ {
		task := &testTask{
			key:        "task" + strconv.Itoa(i),
			expireTime: time.Now().Add(time.Duration(i) * time.Second).UnixMicro(),
		}
		tw.Add(task)
	}

	var wg sync.WaitGroup
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tw.Remove("task" + strconv.Itoa(id))
		}(i)
	}

	wg.Wait()

	size := tw.Size()
	if size > numTasks/10 {
		t.Errorf("Expected size close to 0 after concurrent remove, got %d", size)
	}
}

func TestPassiveScheduler_ConcurrentGetExpired(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	now := time.Now()

	for i := 0; i < 50; i++ {
		task := &testTask{
			key:        "task" + strconv.Itoa(i),
			expireTime: now.Add(time.Duration(i-25) * time.Second).UnixMicro(),
		}
		tw.Add(task)
	}

	var wg sync.WaitGroup
	numReaders := 10

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			expired := tw.GetExpired(now.UnixMicro())
			if len(expired) < 20 || len(expired) > 30 {
				t.Logf("Got %d expired tasks (expected around 25)", len(expired))
			}
		}()
	}

	wg.Wait()
}

func TestPassiveScheduler_MicrosecondPrecision(t *testing.T) {
	tw := NewPassiveScheduler(1 * time.Second)

	now := time.Now()
	microsecondDelay := 500 * time.Microsecond

	task := &testTask{
		key:        "microtask",
		expireTime: now.Add(microsecondDelay).UnixMicro(),
	}

	tw.Add(task)

	expired := tw.GetExpired(now.UnixMicro())
	if len(expired) != 0 {
		t.Errorf("Expected 0 expired tasks immediately, got %d", len(expired))
	}

	time.Sleep(1 * time.Millisecond)
	expired = tw.GetExpired(time.Now().UnixMicro())
	if len(expired) != 1 {
		t.Errorf("Expected 1 expired task after delay, got %d", len(expired))
	}
}

func TestPassiveScheduler_DefaultValues(t *testing.T) {
	tw1 := NewPassiveScheduler(0)
	if tw1.interval == 0 {
		t.Error("Expected default interval, got 0")
	}

	tw2 := NewPassiveScheduler(-1 * time.Second)
	if tw2.interval <= 0 {
		t.Error("Expected positive interval, got non-positive")
	}
}

func TestActiveScheduler_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var callbackCount atomic.Int32
	callback := func(key string, task Task) {
		callbackCount.Add(1)
	}

	tw := NewActiveScheduler(ctx, 50*time.Millisecond, callback)

	task := &testTask{
		key:        "task1",
		expireTime: time.Now().Add(100 * time.Millisecond).UnixMicro(),
	}

	tw.Add(task)
	tw.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	tw.Stop()

	if callbackCount.Load() == 0 {
		t.Error("Expected callback to be called at least once")
	}
}

func BenchmarkPassiveScheduler_Add(b *testing.B) {
	tw := NewPassiveScheduler(1 * time.Second)
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := &testTask{
			key:        "task" + strconv.Itoa(i),
			expireTime: now.Add(time.Duration(i) * time.Second).UnixMicro(),
		}
		tw.Add(task)
	}
}

func BenchmarkPassiveScheduler_GetExpiredNoDue(b *testing.B) {
	tw := NewPassiveScheduler(1 * time.Second)
	now := time.Now()

	for i := 0; i < 1000; i++ {
		task := &testTask{
			key:        "task" + strconv.Itoa(i),
			expireTime: now.Add(time.Duration(i+1) * time.Second).UnixMicro(),
		}
		tw.Add(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tw.GetExpired(now.UnixMicro())
	}
}

func BenchmarkPassiveScheduler_GetExpiredAllDue(b *testing.B) {
	tw := NewPassiveScheduler(1 * time.Second)
	now := time.Now()

	for i := 0; i < 1000; i++ {
		task := &testTask{
			key:        "task" + strconv.Itoa(i),
			expireTime: now.Add(-time.Duration(i+1) * time.Second).UnixMicro(),
		}
		tw.Add(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tw.GetExpired(now.UnixMicro())
	}
}

func BenchmarkPassiveScheduler_Remove(b *testing.B) {
	tw := NewPassiveScheduler(1 * time.Second)
	now := time.Now()

	for i := 0; i < b.N; i++ {
		task := &testTask{
			key:        "task" + strconv.Itoa(i),
			expireTime: now.Add(time.Duration(i) * time.Second).UnixMicro(),
		}
		tw.Add(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tw.Remove("task" + strconv.Itoa(i))
	}
}
