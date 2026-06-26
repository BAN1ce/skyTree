package memory

import (
	"context"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

func init() {
	if logger.Logger == nil {
		logger.LoadForTest()
	}
}

func TestSaveOfflineStateDeletesSessionWhenExpiryIsZero(t *testing.T) {
	core := NewCore()
	ctx := context.Background()
	now := time.Unix(100, 0).UnixNano()

	if _, err := core.OpenSessionForConnect(ctx, &proto_session.OpenSessionForConnectRequest{
		ClientID:              "client-a",
		SessionExpiryInterval: 60,
		NowUnixNano:           now,
	}); err != nil {
		t.Fatalf("open session: %v", err)
	}

	if err := core.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              "client-a",
		SessionExpiryInterval: 0,
		NowUnixNano:           now + int64(time.Second),
		ClearWill:             true,
	}); err != nil {
		t.Fatalf("save offline: %v", err)
	}

	resp, err := core.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: "client-a"})
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if resp.GetExist() {
		t.Fatalf("expected session to be deleted when expiry is zero")
	}
}

func TestReadSessionHidesExpiredOfflineSessionWithoutDeletingLocally(t *testing.T) {
	core := NewCore()
	ctx := context.Background()
	now := time.Unix(100, 0).UnixNano()

	if _, err := core.OpenSessionForConnect(ctx, &proto_session.OpenSessionForConnectRequest{
		ClientID:              "client-a",
		SessionExpiryInterval: 1,
		NowUnixNano:           now,
	}); err != nil {
		t.Fatalf("open session: %v", err)
	}
	if err := core.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              "client-a",
		SessionExpiryInterval: 1,
		NowUnixNano:           now,
	}); err != nil {
		t.Fatalf("save offline: %v", err)
	}

	resp, err := core.GetSession(ctx, &proto_session.ReadSessionRequest{
		ClientID:    "client-a",
		NowUnixNano: now + int64(2*time.Second),
	})
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if resp.GetExist() {
		t.Fatalf("expected expired session to be hidden")
	}

	deleted, err := core.DeleteExpiredSessions(ctx, now+int64(2*time.Second))
	if err != nil {
		t.Fatalf("delete expired sessions: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "client-a" {
		t.Fatalf("expected replicated cleanup to delete client-a, got %v", deleted)
	}
}

func TestSaveOfflineStatePersistsAndOverwritesOutgoingReplayCursor(t *testing.T) {
	core := NewCore()
	ctx := context.Background()
	clientID := "client-a"
	now := time.Unix(100, 0).UnixNano()
	cursor := &proto_session.OutgoingReplayCursor{
		TaskUnixMicro: time.Unix(100, 5000).UnixMicro(),
		TaskID:        "00000000-0000-0000-0000-000000000501",
		Generation:    2,
	}

	if err := core.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              clientID,
		SessionExpiryInterval: 60,
		NowUnixNano:           now,
		OutgoingReplayCursor:  cursor,
	}); err != nil {
		t.Fatalf("save offline with replay cursor: %v", err)
	}

	resp, err := core.GetSession(ctx, &proto_session.ReadSessionRequest{
		ClientID:    clientID,
		NowUnixNano: now + int64(time.Second),
	})
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	got := resp.GetSession().GetOutgoingReplayCursor()
	if got == nil {
		t.Fatal("expected replay cursor to be saved")
	}
	if got.GetTaskID() != cursor.GetTaskID() || got.GetTaskUnixMicro() != cursor.GetTaskUnixMicro() || got.GetGeneration() != cursor.GetGeneration() {
		t.Fatalf("saved replay cursor mismatch: got %+v want %+v", got, cursor)
	}

	if err := core.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              clientID,
		SessionExpiryInterval: 60,
		NowUnixNano:           now + int64(time.Second),
	}); err != nil {
		t.Fatalf("overwrite offline without replay cursor: %v", err)
	}

	resp, err = core.GetSession(ctx, &proto_session.ReadSessionRequest{
		ClientID:    clientID,
		NowUnixNano: now + int64(2*time.Second),
	})
	if err != nil {
		t.Fatalf("read overwritten session: %v", err)
	}
	if got := resp.GetSession().GetOutgoingReplayCursor(); got != nil {
		t.Fatalf("expected replay cursor to be cleared by snapshot overwrite, got %+v", got)
	}
}

func TestDeleteExpiredSessionsRemovesOnlyExpiredOfflineSessions(t *testing.T) {
	core := NewCore()
	ctx := context.Background()
	now := time.Unix(100, 0).UnixNano()

	for _, clientID := range []string{"expired-a", "active-b"} {
		if _, err := core.OpenSessionForConnect(ctx, &proto_session.OpenSessionForConnectRequest{
			ClientID:              clientID,
			SessionExpiryInterval: 1,
			NowUnixNano:           now,
		}); err != nil {
			t.Fatalf("open session %s: %v", clientID, err)
		}
	}
	if err := core.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              "expired-a",
		SessionExpiryInterval: 1,
		NowUnixNano:           now,
	}); err != nil {
		t.Fatalf("save expired offline: %v", err)
	}
	if err := core.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              "active-b",
		SessionExpiryInterval: 1,
		NowUnixNano:           now + int64(2*time.Second),
	}); err != nil {
		t.Fatalf("save active offline: %v", err)
	}

	deleted, err := core.DeleteExpiredSessions(ctx, now+int64(1500*time.Millisecond))
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "expired-a" {
		t.Fatalf("expected only expired-a deleted, got %v", deleted)
	}

	resp, err := core.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: "expired-a"})
	if err != nil {
		t.Fatalf("get expired session: %v", err)
	}
	if resp.GetExist() {
		t.Fatalf("expired session should be deleted")
	}
	resp, err = core.GetSession(ctx, &proto_session.ReadSessionRequest{
		ClientID:    "active-b",
		NowUnixNano: now + int64(1500*time.Millisecond),
	})
	if err != nil {
		t.Fatalf("get active session: %v", err)
	}
	if !resp.GetExist() {
		t.Fatalf("active session should remain")
	}
}

func TestSaveOfflineStateIgnoresStaleOwnerToken(t *testing.T) {
	core := NewCore()
	ctx := context.Background()
	now := time.Unix(100, 0).UnixNano()
	clientID := "client-a"

	if _, err := core.OpenSessionForConnect(ctx, &proto_session.OpenSessionForConnectRequest{
		ClientID:              clientID,
		SessionExpiryInterval: 60,
		NowUnixNano:           now,
	}); err != nil {
		t.Fatalf("open session: %v", err)
	}
	if _, err := core.TakeOverSessionOwner(ctx, &proto_session.TakeOverSessionOwnerRequest{
		Owner: &proto_session.SessionOwner{
			ClientID:   clientID,
			NodeID:     1,
			OwnerToken: "new-token",
			Online:     true,
		},
	}); err != nil {
		t.Fatalf("take over owner: %v", err)
	}

	if err := core.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              clientID,
		OwnerToken:            "old-token",
		SessionExpiryInterval: 60,
		NowUnixNano:           now + int64(time.Second),
		OutgoingReplayCursor: &proto_session.OutgoingReplayCursor{
			TaskUnixMicro: time.Unix(100, 0).UnixMicro(),
			TaskID:        "00000000-0000-0000-0000-000000000502",
			Generation:    1,
		},
	}); err != nil {
		t.Fatalf("save offline with stale owner token: %v", err)
	}

	ownerResp, err := core.GetSessionOwner(ctx, &proto_session.ReadSessionOwnerRequest{ClientID: clientID})
	if err != nil {
		t.Fatalf("read owner: %v", err)
	}
	if !ownerResp.GetExist() || ownerResp.GetOwner().GetOwnerToken() != "new-token" || !ownerResp.GetOwner().GetOnline() {
		t.Fatalf("expected new owner to stay online, got %+v", ownerResp.GetOwner())
	}

	sessionResp, err := core.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: clientID})
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if len(sessionResp.GetSession().GetUnfinishedMessages()) != 0 {
		t.Fatalf("stale offline state should not overwrite session, got %v", sessionResp.GetSession().GetUnfinishedMessages())
	}
	if cursor := sessionResp.GetSession().GetOutgoingReplayCursor(); cursor != nil {
		t.Fatalf("stale offline state should not save replay cursor, got %+v", cursor)
	}
}
