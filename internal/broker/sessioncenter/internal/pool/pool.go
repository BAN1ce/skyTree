package pool

import (
	"sync"

	"github.com/BAN1ce/skyTree/proto/proto_session"
)

var (
	RawRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.SessionRequest{}
		},
	}

	AddSessionRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.OpenSessionForConnectRequest{}
		},
	}

	TakeOverSessionOwnerRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.TakeOverSessionOwnerRequest{}
		},
	}

	ReplaceSessionStateOnCleanStartRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.ReplaceSessionStateOnCleanStartRequest{}
		},
	}

	DeleteSessionRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.DeleteSessionRequest{}
		},
	}

	UpdateSessionRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.SaveOfflineStateRequest{}
		},
	}

	RemoveOutgoingUnfinishedRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.RemoveOutgoingUnfinishedRequest{}
		},
	}

	UpsertIncomingUnfinishedRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.UpsertIncomingUnfinishedRequest{}
		},
	}

	RemoveIncomingUnfinishedRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.RemoveIncomingUnfinishedRequest{}
		},
	}

	ReadSessionRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.ReadSessionRequest{}
		},
	}

	ReadSessionOwnerRequest = &sync.Pool{
		New: func() interface{} {
			return &proto_session.ReadSessionOwnerRequest{}
		},
	}
)
