package wal

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLocalWALCenter_RestartRecovery(t *testing.T) {
	if logger.Logger == nil {
		logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	}
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "session_center")

	c1, err := NewLocalWALCenter(baseDir, 0, 1)
	require.NoError(t, err)

	ctx := context.TODO()
	_, err = c1.OpenSessionForConnect(ctx, &proto_session.OpenSessionForConnectRequest{
		ClientID: "c1",
		WillMessage: &proto_session.WillMessage{
			Topic: "will/topic",
		},
	})
	require.NoError(t, err)
	_, err = c1.TakeOverSessionOwner(ctx, &proto_session.TakeOverSessionOwnerRequest{
		Owner: &proto_session.SessionOwner{
			ClientID:   "c1",
			NodeID:     1,
			OwnerToken: "tok1",
			Online:     true,
		},
	})
	require.NoError(t, err)
	require.NoError(t, c1.Close())

	c2, err := NewLocalWALCenter(baseDir, 0, 1)
	require.NoError(t, err)
	defer func() { _ = c2.Close() }()

	resp, err := c2.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: "c1"})
	require.NoError(t, err)
	require.True(t, resp.GetExist())
	require.Equal(t, "c1", resp.GetSession().GetClientID())
	require.Equal(t, "will/topic", resp.GetSession().GetWillMessage().GetTopic())

	ownerResp, err := c2.GetSessionOwner(ctx, &proto_session.ReadSessionOwnerRequest{ClientID: "c1"})
	require.NoError(t, err)
	require.True(t, ownerResp.GetExist())
	require.Equal(t, "tok1", ownerResp.GetOwner().GetOwnerToken())
	require.True(t, ownerResp.GetOwner().GetOnline())

	ownerBatchResp, err := c2.GetSessionOwners(ctx, &proto_session.ReadSessionOwnersRequest{
		ClientIDs: []string{"c1", "missing", "c1"},
	})
	require.NoError(t, err)
	require.Len(t, ownerBatchResp.GetItems(), 2)
	require.Equal(t, "c1", ownerBatchResp.GetItems()[0].GetClientID())
	require.True(t, ownerBatchResp.GetItems()[0].GetExist())
	require.Equal(t, "missing", ownerBatchResp.GetItems()[1].GetClientID())
	require.False(t, ownerBatchResp.GetItems()[1].GetExist())

	require.NoError(t, c2.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              "c1",
		SessionExpiryInterval: 60,
		UnfinishedMessages: []*proto_session.UnfinishedMessage{
			{MessageID: "m1", PacketID: 1, Qos: 2, State: proto_session.UnfinishedMessage_WAITING_PUBREC, IsOutgoing: true},
		},
		OutgoingReplayCursor: &proto_session.OutgoingReplayCursor{
			TaskUnixMicro: time.Unix(100, 0).UnixMicro(),
			TaskID:        "00000000-0000-0000-0000-000000000601",
			Generation:    1,
		},
		ClearWill: true,
	}))
	require.NoError(t, c2.Close())

	c3, err := NewLocalWALCenter(baseDir, time.Second, 10000)
	require.NoError(t, err)
	defer func() { _ = c3.Close() }()

	resp2, err := c3.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: "c1"})
	require.NoError(t, err)
	require.True(t, resp2.GetExist())
	require.Len(t, resp2.GetSession().GetUnfinishedMessages(), 1)
	require.Equal(t, "00000000-0000-0000-0000-000000000601", resp2.GetSession().GetOutgoingReplayCursor().GetTaskID())
	require.Nil(t, resp2.GetSession().GetWillMessage())

	ownerResp2, err := c3.GetSessionOwner(ctx, &proto_session.ReadSessionOwnerRequest{ClientID: "c1"})
	require.NoError(t, err)
	require.True(t, ownerResp2.GetExist())
	require.False(t, ownerResp2.GetOwner().GetOnline())
}

func TestLocalWALCenter_DeleteExpiredSessionsPersistsDeletion(t *testing.T) {
	if logger.Logger == nil {
		logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	}
	ctx := context.Background()
	baseDir := filepath.Join(t.TempDir(), "session_center")
	now := time.Unix(100, 0).UnixNano()

	c1, err := NewLocalWALCenter(baseDir, 0, 10000)
	require.NoError(t, err)
	_, err = c1.OpenSessionForConnect(ctx, &proto_session.OpenSessionForConnectRequest{
		ClientID:              "expired-a",
		SessionExpiryInterval: 1,
		NowUnixNano:           now,
	})
	require.NoError(t, err)
	require.NoError(t, c1.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              "expired-a",
		SessionExpiryInterval: 1,
		NowUnixNano:           now,
	}))
	deleted, err := c1.DeleteExpiredSessions(ctx, now+int64(2*time.Second))
	require.NoError(t, err)
	require.Equal(t, []string{"expired-a"}, deleted)
	require.NoError(t, c1.Close())

	c2, err := NewLocalWALCenter(baseDir, 0, 10000)
	require.NoError(t, err)
	defer func() { _ = c2.Close() }()
	resp, err := c2.GetSession(ctx, &proto_session.ReadSessionRequest{ClientID: "expired-a"})
	require.NoError(t, err)
	require.False(t, resp.GetExist())
}
