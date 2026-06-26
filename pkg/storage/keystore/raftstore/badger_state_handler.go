package raftstore

import (
	"context"
	"fmt"
	"io"
	"time"

	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/cluster/dbpb"
	"github.com/lni/dragonboat/v3/statemachine"
)

type StateHandler struct {
	db store.KeyStoreWithBackup
}

func NewStateHandler(s store.KeyStoreWithBackup) *StateHandler {
	return &StateHandler{
		db: s,
	}
}

func (s *StateHandler) HandleWrite(ctx context.Context, request *dbpb.Request) (statemachine.Result, error) {
	var (
		result statemachine.Result
		err    error
		cmd    = request.GetCMD()
	)

	switch request.Type {
	case dbpb.DB_REQUEST_TYPE_PUT:
		err = s.db.PutKey(ctx, cmd[0], cmd[1])

	case dbpb.DB_REQUEST_TYPE_DELETE:
		err = s.db.DeleteKey(ctx, cmd[0])

	case dbpb.DB_REQUEST_TYPE_DELETE_PREFIX:
		err = s.db.DeletePrefixKey(ctx, cmd[0])

	case dbpb.DB_REQUEST_TYPE_Z_ADD:

	case dbpb.DB_REQUEST_TYPE_Z_DELETE:

	case dbpb.DB_REQUEST_TYPE_HSET:

		err = s.db.HSet(ctx, cmd[0], cmd[1:])

	case dbpb.DB_REQUEST_TYPE_HDEL:
		err = s.db.HDel(ctx, cmd[0], cmd[1:])

	case dbpb.DB_REQUEST_TYPE_DELETE_HASH:
		err = s.db.DeleteHash(ctx, cmd[0])

	case dbpb.DB_REQUEST_TYPE_EXPIRE:
		expirer, ok := s.db.(store.Expirer)
		if !ok {
			err = fmt.Errorf("keystore does not support expiration")
			break
		}
		if len(cmd) < 1 {
			err = fmt.Errorf("invalid EXPIRE request: missing key")
			break
		}
		if request.GetTtlNanos() <= 0 {
			err = fmt.Errorf("invalid EXPIRE request: ttl_nanos must be > 0, got %d", request.GetTtlNanos())
			break
		}
		err = expirer.SetExpired(ctx, cmd[0], time.Duration(request.GetTtlNanos()))

	default:
		err = fmt.Errorf("invalid request type")

	}
	return result, err

}

func (s *StateHandler) HandleRead(ctx context.Context, request *dbpb.Request) (interface{}, error) {
	var (
		result    = new(dbpb.Response)
		err       error
		value     []byte
		ok        bool
		getPrefix map[string]string
	)

	switch request.GetType() {
	case dbpb.DB_REQUEST_TYPE_GET:
		value, ok, err = s.db.ReadKey(ctx, request.GetCMD()[0])
		if ok {
			result.Exist = true
		}
		result.Result = append(result.Result, value)
	//case dbpb.DB_REQUEST_TYPE_GET_PREFIX:
	//  getPrefix, err = s.db.ReadPrefixKey(ctx, request.GetCMD()[0])
	//  result.Result = prefixValueToResult(getPrefix)

	case dbpb.DB_REQUEST_TYPE_HGET:
		value, ok, err = s.db.HGet(ctx, request.GetCMD()[0], request.GetCMD()[1])
		result.Exist = ok
		if ok {
			result.Result = append(result.Result, value)
		}

	case dbpb.DB_REQUEST_TYPE_HGET_ALL:
		getPrefix, err = s.db.HGetAll(ctx, request.GetCMD()[0])
		result.Result = prefixValueToResult(getPrefix)

	case dbpb.DB_REQUEST_TYPE_HGET_PREFIX:
		getPrefix, err = s.db.HPrefix(ctx, request.GetCMD()[0], request.GetCMD()[1])
		result.Result = prefixValueToResult(getPrefix)

	default:
		err = fmt.Errorf("invalid request type")

	}

	return result, err

}
func prefixValueToResult(a map[string]string) [][]byte {
	var result [][]byte
	for k, v := range a {
		if v == "" {
			continue
		}
		if k == "" {
			continue
		}
		result = append(result, []byte(k), []byte(v))
	}
	return result

}

func (s *StateHandler) SaveSnapshot(writer io.Writer, collection statemachine.ISnapshotFileCollection, i <-chan struct{}) error {
	return s.db.Snapshot(writer)
}

func (s *StateHandler) RecoverFromSnapshot(reader io.Reader, files []statemachine.SnapshotFile, i <-chan struct{}) error {
	return s.db.Recover(reader)
}

func (s *StateHandler) Close() error {
	return s.db.Close()
}
