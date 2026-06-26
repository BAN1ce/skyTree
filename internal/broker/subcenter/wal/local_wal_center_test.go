package wal

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/BAN1ce/skyTree/logger"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLocalWALCenter_RestartRecovery(t *testing.T) {
	if logger.Logger == nil {
		logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	}
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "sub_center")

	c1, err := NewLocalWALCenter(baseDir, 0, 1)
	require.NoError(t, err)

	ctx := context.TODO()
	_, err = c1.SetClientOwnerToken(ctx, &proto.SetClientOwnerTokenRequest{
		ClientID:   "c1",
		OwnerToken: "tok1",
	})
	require.NoError(t, err)

	_, err = c1.CreateSub(ctx, &proto.SubRequest{
		ClientID:   "c1",
		OwnerToken: "tok1",
		Topics: []*proto.SubOption{
			{Topic: "/a/b", QoS: 1},
		},
	})
	require.NoError(t, err)
	require.NoError(t, c1.Close())

	// Restart and verify subscription is recovered.
	c2, err := NewLocalWALCenter(baseDir, 0, 1)
	require.NoError(t, err)
	defer func() { _ = c2.Close() }()

	subResp, err := c2.GetClientSubscriptions(ctx, &proto.GetClientSubscriptionsRequest{ClientID: "c1"})
	require.NoError(t, err)
	require.Contains(t, subResp.GetTopics(), "/a/b")
	require.EqualValues(t, 1, subResp.GetTopics()["/a/b"].GetQoS())
}
