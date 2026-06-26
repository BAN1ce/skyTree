package willdelay

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
)

func TestScannerTreatsNilClusterAsSingleNodeLeader(t *testing.T) {
	scanner := NewScanner(context.Background(), &fakeWillDelayCenter{}, nil, nil, nil, 0, 0)
	if !scanner.isCurrentLeader() {
		t.Fatal("expected nil cluster scanner to act as single-node leader")
	}
}

func TestScannerChecksCurrentLeaderEveryCycle(t *testing.T) {
	scanner := NewScanner(context.Background(), &fakeWillDelayCenter{}, nil, nil, nil, 4, 2)
	reader := &fakeWillDelayLeaderReader{leaderID: 2, valid: true}
	scanner.cluster = reader

	if !scanner.isCurrentLeader() {
		t.Fatal("expected scanner to become leader when local node is raft leader")
	}

	reader.leaderID = 3
	if scanner.isCurrentLeader() {
		t.Fatal("expected scanner to stop being leader after raft leader changes")
	}
}

func TestScannerLeaderCheckErrorSkipsCurrentCycle(t *testing.T) {
	scanner := NewScanner(context.Background(), &fakeWillDelayCenter{}, nil, nil, nil, 4, 2)
	scanner.cluster = &fakeWillDelayLeaderReader{err: errLeaderUnavailable}

	if scanner.isCurrentLeader() {
		t.Fatal("expected leader check error to skip current scan cycle")
	}
}

func TestScannerSkipsDueTaskWhenOwnerTokenIsStale(t *testing.T) {
	logger.LoadForTest()

	ctx := context.Background()
	center := &fakeWillDelayCenter{}
	clientID := "client-a"
	if err := center.AddTask(ctx, &proto_will_delay.WillDelayTask{
		ClientID:             clientID,
		OwnerToken:           "old-token",
		ScheduledPublishTime: time.Now().Add(-time.Second).UnixMicro(),
	}); err != nil {
		t.Fatalf("add task: %v", err)
	}

	sessionCenter := &fakeWillDelaySessionCenter{
		sessionResp: &proto_session.ReadSessionResponse{
			Exist: true,
			Session: &proto_session.Session{
				ClientID: clientID,
				WillMessage: &proto_session.WillMessage{
					Topic:   "will/topic",
					Payload: []byte("stale"),
					Qos:     1,
				},
			},
		},
		ownerResp: &proto_session.ReadSessionOwnerResponse{
			Exist: true,
			Owner: &proto_session.SessionOwner{
				ClientID:   clientID,
				OwnerToken: "new-token",
				Online:     true,
			},
		},
	}
	willCh := make(chan *brokerpublish.Message, 1)
	scanner := NewScanner(ctx, center, sessionCenter, willCh, nil, 0, 0)

	scanner.scanAndExecute()

	select {
	case msg := <-willCh:
		t.Fatalf("stale will delay task must not publish will: %+v", msg)
	default:
	}
	dueTasks, err := center.GetDueTasks(ctx, time.Now().UnixMicro())
	if err != nil {
		t.Fatalf("get due tasks: %v", err)
	}
	if len(dueTasks) != 0 {
		t.Fatalf("stale will delay task should be deleted after skip, got %v", dueTasks)
	}
}

type fakeWillDelayLeaderReader struct {
	leaderID uint64
	valid    bool
	err      error
}

var errLeaderUnavailable = errors.New("leader unavailable")

func (f *fakeWillDelayLeaderReader) GetLeader(uint64) (uint64, bool, error) {
	return f.leaderID, f.valid, f.err
}

type fakeWillDelayCenter struct {
	tasks []*proto_will_delay.WillDelayTask
}

func (f *fakeWillDelayCenter) AddTask(ctx context.Context, task *proto_will_delay.WillDelayTask) error {
	_ = ctx
	f.tasks = append(f.tasks, task)
	return nil
}

func (f *fakeWillDelayCenter) DeleteTask(ctx context.Context, clientID string) error {
	_ = ctx
	remaining := f.tasks[:0]
	for _, task := range f.tasks {
		if task.GetClientID() != clientID {
			remaining = append(remaining, task)
		}
	}
	f.tasks = remaining
	return nil
}

func (f *fakeWillDelayCenter) DeleteTaskByOwner(ctx context.Context, clientID, ownerToken string) error {
	_ = ctx
	remaining := f.tasks[:0]
	for _, task := range f.tasks {
		if task.GetClientID() == clientID && task.GetOwnerToken() == ownerToken {
			continue
		}
		remaining = append(remaining, task)
	}
	f.tasks = remaining
	return nil
}

func (f *fakeWillDelayCenter) GetDueTasks(
	ctx context.Context,
	nowUnixMicro int64,
) ([]*proto_will_delay.WillDelayTask, error) {
	_ = ctx
	dueTasks := make([]*proto_will_delay.WillDelayTask, 0, len(f.tasks))
	for _, task := range f.tasks {
		if task.GetScheduledPublishTime() <= nowUnixMicro {
			dueTasks = append(dueTasks, task)
		}
	}
	return dueTasks, nil
}

type fakeWillDelaySessionCenter struct {
	sessionResp *proto_session.ReadSessionResponse
	ownerResp   *proto_session.ReadSessionOwnerResponse
}

func (f *fakeWillDelaySessionCenter) OpenSessionForConnect(context.Context, *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error) {
	return nil, nil
}

func (f *fakeWillDelaySessionCenter) TakeOverSessionOwner(context.Context, *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error) {
	return nil, nil
}

func (f *fakeWillDelaySessionCenter) ReplaceSessionStateOnCleanStart(context.Context, *proto_session.ReplaceSessionStateOnCleanStartRequest) error {
	return nil
}

func (f *fakeWillDelaySessionCenter) SaveOfflineState(context.Context, *proto_session.SaveOfflineStateRequest) error {
	return nil
}

func (f *fakeWillDelaySessionCenter) RemoveOutgoingUnfinished(context.Context, *proto_session.RemoveOutgoingUnfinishedRequest) error {
	return nil
}

func (f *fakeWillDelaySessionCenter) UpsertIncomingUnfinished(context.Context, *proto_session.UpsertIncomingUnfinishedRequest) error {
	return nil
}

func (f *fakeWillDelaySessionCenter) RemoveIncomingUnfinished(context.Context, *proto_session.RemoveIncomingUnfinishedRequest) error {
	return nil
}

func (f *fakeWillDelaySessionCenter) CommitOutgoingProgress(context.Context, *proto_session.CommitOutgoingProgressRequest) error {
	return nil
}

func (f *fakeWillDelaySessionCenter) DeleteSession(context.Context, *proto_session.DeleteSessionRequest) error {
	return nil
}

func (f *fakeWillDelaySessionCenter) GetSession(context.Context, *proto_session.ReadSessionRequest) (*proto_session.ReadSessionResponse, error) {
	return f.sessionResp, nil
}

func (f *fakeWillDelaySessionCenter) GetSessionOwner(context.Context, *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error) {
	return f.ownerResp, nil
}

func (f *fakeWillDelaySessionCenter) GetSessionOwners(context.Context, *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error) {
	return &proto_session.ReadSessionOwnersResponse{Items: []*proto_session.ReadSessionOwnerItem{}}, nil
}
