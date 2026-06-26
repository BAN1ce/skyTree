package memory

import (
	"context"
	"io"
	"testing"

	"github.com/BAN1ce/skyTree/logger"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestMemorySubCenter_OwnerTokenFencing_SubAndDelete(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	center := NewMemorySubCenter()
	ctx := context.TODO()

	// Establish ownership.
	setResp, err := center.SetClientOwnerToken(ctx, &proto.SetClientOwnerTokenRequest{
		ClientID:   "c1",
		OwnerToken: "t1",
	})
	require.NoError(t, err)
	require.True(t, setResp.GetSuccess())

	// Subscribe with correct token.
	_, err = center.CreateSub(ctx, &proto.SubRequest{
		ClientID:   "c1",
		OwnerToken: "t1",
		Topics: []*proto.SubOption{
			{Topic: "a/b", QoS: 1},
		},
	})
	require.NoError(t, err)

	// Mismatched token must be rejected.
	rsp, err := center.CreateSub(ctx, &proto.SubRequest{
		ClientID:   "c1",
		OwnerToken: "t2",
		Topics: []*proto.SubOption{
			{Topic: "a/c", QoS: 1},
		},
	})
	require.NoError(t, err)
	require.False(t, rsp.GetSuccess())
	require.Equal(t, int32(-1), rsp.GetTopics()["a/c"])

	// Delete with mismatched token should be ignored (Success=false).
	delResp, err := center.DeleteClient(ctx, &proto.DeleteClientRequest{
		ClientID:   "c1",
		OwnerToken: "t2",
	})
	require.NoError(t, err)
	require.False(t, delResp.GetSuccess())

	// Delete with correct token should remove subscriptions.
	delResp, err = center.DeleteClient(ctx, &proto.DeleteClientRequest{
		ClientID:   "c1",
		OwnerToken: "t1",
	})
	require.NoError(t, err)
	require.True(t, delResp.GetSuccess())

	match, err := center.GetAllMatchClient(ctx, &proto.GetAllSubTopicClientRequest{Topic: "a/b"})
	require.NoError(t, err)
	require.Empty(t, match.GetClient())
}
