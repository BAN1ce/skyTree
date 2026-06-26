package scyllastore

import (
	"context"
	"testing"
	"time"

	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/google/uuid"
)

func TestBucketStartNanoRoundsDownToBucketDuration(t *testing.T) {
	got := bucketStartNano(time.Unix(3661, 123), time.Hour)
	want := time.Unix(3600, 0).UnixNano()
	if got != want {
		t.Fatalf("bucketStartNano = %d, want %d", got, want)
	}
}

func TestDeliveryCursorForwardOnly(t *testing.T) {
	current := &brokerstore.DeliveryCursor{
		Generation: 2,
		LastTS:     time.Unix(10, 0),
		LastTaskID: uuid.MustParse("00000000-0000-0000-0000-000000000010"),
	}

	tests := []struct {
		name string
		next brokerstore.DeliveryCursor
		want bool
	}{
		{
			name: "newer timestamp advances",
			next: brokerstore.DeliveryCursor{
				Generation: 2,
				LastTS:     time.Unix(11, 0),
				LastTaskID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			},
			want: true,
		},
		{
			name: "same timestamp and greater task id advances",
			next: brokerstore.DeliveryCursor{
				Generation: 2,
				LastTS:     time.Unix(10, 0),
				LastTaskID: uuid.MustParse("00000000-0000-0000-0000-000000000011"),
			},
			want: true,
		},
		{
			name: "older timestamp is stale",
			next: brokerstore.DeliveryCursor{
				Generation: 2,
				LastTS:     time.Unix(9, 0),
				LastTaskID: uuid.MustParse("00000000-0000-0000-0000-000000000999"),
			},
			want: false,
		},
		{
			name: "stale generation is rejected even when timestamp is newer",
			next: brokerstore.DeliveryCursor{
				Generation: 1,
				LastTS:     time.Unix(11, 0),
				LastTaskID: uuid.MustParse("00000000-0000-0000-0000-000000000999"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deliveryCursorAfter(tt.next, current); got != tt.want {
				t.Fatalf("deliveryCursorAfter = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppendDeliveryTaskReportsDuplicateFromDedupeCAS(t *testing.T) {
	session := &fakeSession{casApplied: false}
	store := newStoreWithSession(session, StoreOptions{BucketDuration: time.Hour})

	inserted, err := store.AppendDeliveryTask(context.Background(), brokerstore.DeliveryTask{
		TS:          time.Unix(10, 0),
		TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000101"),
		ClientID:    "client-a",
		MessageID:   uuid.MustParse("00000000-0000-0000-0000-000000000201"),
		DeliveryQoS: 1,
	})
	if err != nil {
		t.Fatalf("AppendDeliveryTask error: %v", err)
	}
	if inserted {
		t.Fatal("AppendDeliveryTask inserted = true, want false")
	}
	if session.execCount != 0 {
		t.Fatalf("task rows should not be inserted for duplicate CAS, execCount=%d", session.execCount)
	}
}

type fakeSession struct {
	casApplied bool
	execCount  int
}

func (s *fakeSession) Query(stmt string, values ...any) cqlQuery {
	return &fakeQuery{session: s}
}

func (s *fakeSession) Close() {}

type fakeQuery struct {
	session *fakeSession
}

func (q *fakeQuery) WithContext(context.Context) cqlQuery {
	return q
}

func (q *fakeQuery) Exec() error {
	q.session.execCount++
	return nil
}

func (q *fakeQuery) Scan(dest ...any) error {
	return errNotFound
}

func (q *fakeQuery) Iter() cqlIter {
	return &fakeIter{}
}

func (q *fakeQuery) MapScanCAS(map[string]any) (bool, error) {
	return q.session.casApplied, nil
}

type fakeIter struct{}

func (i *fakeIter) Scan(dest ...any) bool {
	return false
}

func (i *fakeIter) Close() error {
	return nil
}
