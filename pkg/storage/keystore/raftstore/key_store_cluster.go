package raftstore

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/pkg/cluster/dbpb"
	"google.golang.org/protobuf/proto"
)

//
// ----------------------------------------------------------------- KeyStoreCluster ----------------------------------------------------------------- //
//

type KeyStoreCluster struct {
	client cluster.Client
	bucket string
}

func NewKeyStoreCluster(client cluster.Client) *KeyStoreCluster {
	return &KeyStoreCluster{
		client: client,
	}
}

func (n *KeyStoreCluster) PutKey(ctx context.Context, key, value []byte) error {
	var (
		req = protoDBRequestPool.Get()
	)
	defer protoDBRequestPool.Put(req)

	req.Type = dbpb.DB_REQUEST_TYPE_PUT
	req.CMD = [][]byte{key, value}
	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = n.client.Write(ctx, data)
	return err

}

func (n *KeyStoreCluster) ReadKey(ctx context.Context, key []byte) ([]byte, bool, error) {
	var (
		req = protoDBRequestPool.Get()
	)
	defer protoDBRequestPool.Put(req)

	req.Type = dbpb.DB_REQUEST_TYPE_GET
	req.CMD = [][]byte{key}

	resp, err := n.client.Read(ctx, req)
	if err != nil {
		return nil, false, err
	}

	if resp == nil {
		return nil, false, nil
	}
	if resp, ok := resp.(*dbpb.Response); ok {
		if !resp.Exist {
			return nil, false, nil
		}
		if len(resp.Result) == 0 {
			return nil, false, nil
		}

		return resp.Result[0], true, nil
	}

	return nil, false, fmt.Errorf("unexpected response type %T", resp)

}

func (n *KeyStoreCluster) DeleteKey(ctx context.Context, key []byte) error {
	var (
		req = protoDBRequestPool.Get()
	)
	defer protoDBRequestPool.Put(req)
	req.Type = dbpb.DB_REQUEST_TYPE_DELETE
	req.CMD = [][]byte{key}

	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = n.client.Write(ctx, data)
	return err
}

func (n *KeyStoreCluster) ReadPrefixKey(ctx context.Context, prefix []byte) (map[string]string, error) {
	var (
		req = protoDBRequestPool.Get()
	)
	defer protoDBRequestPool.Put(req)
	req.Type = dbpb.DB_REQUEST_TYPE_GET_PREFIX
	req.CMD = [][]byte{prefix}

	resp, err := n.client.Read(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp, ok := resp.(*dbpb.Response); ok {
		return parseKVResponseResult(resp.Result, "GET_PREFIX")
	}
	return make(map[string]string), nil

}

func (n *KeyStoreCluster) DeletePrefixKey(ctx context.Context, prefix []byte) error {
	var (
		req = protoDBRequestPool.Get()
	)
	defer protoDBRequestPool.Put(req)

	req.Type = dbpb.DB_REQUEST_TYPE_DELETE_PREFIX
	req.CMD = [][]byte{prefix}

	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = n.client.Write(ctx, data)
	return err
}

func (n *KeyStoreCluster) Close() error {
	return nil
}

func (n *KeyStoreCluster) HSet(ctx context.Context, key []byte, field [][]byte) error {
	var (
		req = protoDBRequestPool.Get()
	)
	defer protoDBRequestPool.Put(req)

	req.Type = dbpb.DB_REQUEST_TYPE_HSET
	req.CMD = append(req.CMD, key)
	req.CMD = append(req.CMD, field...)

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()
	_, err = n.client.Write(writeCtx, body)
	return err

}

func (n *KeyStoreCluster) HGet(ctx context.Context, key, field []byte) ([]byte, bool, error) {
	var (
		req = protoDBRequestPool.Get()
	)
	req.Type = dbpb.DB_REQUEST_TYPE_HGET
	req.CMD = [][]byte{key, field}
	defer protoDBRequestPool.Put(req)

	resp, err := n.client.Read(ctx, req)
	if err != nil {
		return nil, false, err
	}

	if resp, ok := resp.(*dbpb.Response); ok {
		if len(resp.Result) == 0 {
			return nil, false, nil
		}
		return resp.Result[0], resp.GetExist(), nil
	}

	return nil, false, fmt.Errorf("unexpected response type %T", resp)
}

func (n *KeyStoreCluster) HDel(ctx context.Context, key []byte, field [][]byte) error {
	var (
		req = protoDBRequestPool.Get()
	)
	req.Type = dbpb.DB_REQUEST_TYPE_HDEL
	req.CMD = [][]byte{key}
	req.CMD = append(req.CMD, field...)

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = n.client.Write(ctx, body)
	return err
}

func (n *KeyStoreCluster) HGetAll(ctx context.Context, key []byte) (map[string]string, error) {
	req := protoDBRequestPool.Get()

	req.Type = dbpb.DB_REQUEST_TYPE_HGET_ALL
	req.CMD = [][]byte{key}

	defer protoDBRequestPool.Put(req)

	resp, err := n.client.Read(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp, ok := resp.(*dbpb.Response); ok {
		return parseKVResponseResult(resp.Result, "HGET_ALL")
	}

	return nil, fmt.Errorf("unexpected response type %T", resp)
}

func (n *KeyStoreCluster) HPrefix(ctx context.Context, key []byte, prefix []byte) (map[string]string, error) {
	var (
		req = protoDBRequestPool.Get()
	)
	req.Type = dbpb.DB_REQUEST_TYPE_HGET_PREFIX
	req.CMD = [][]byte{key, prefix}

	defer protoDBRequestPool.Put(req)
	resp, err := n.client.Read(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp, ok := resp.(*dbpb.Response); ok {
		return parseKVResponseResult(resp.Result, "HGET_PREFIX")
	}

	return nil, fmt.Errorf("unexpected response type %T", resp)
}

func (n *KeyStoreCluster) DeleteHash(ctx context.Context, key []byte) error {
	var (
		req = protoDBRequestPool.Get()
	)
	req.Type = dbpb.DB_REQUEST_TYPE_DELETE_HASH
	req.CMD = [][]byte{key}

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = n.client.Write(ctx, body)
	return err
}

func (n *KeyStoreCluster) Snapshot(writer io.Writer) error {
	return nil
}

func (n *KeyStoreCluster) Recover(reader io.Reader) error {
	return nil
}

func (n *KeyStoreCluster) SetExpired(ctx context.Context, key []byte, duration time.Duration) error {
	if duration <= 0 {
		return fmt.Errorf("set expired: duration must be > 0, got %s", duration)
	}
	var (
		req = protoDBRequestPool.Get()
	)
	defer protoDBRequestPool.Put(req)

	req.Type = dbpb.DB_REQUEST_TYPE_EXPIRE
	req.CMD = [][]byte{key}
	req.TtlNanos = duration.Nanoseconds()

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	_, err = n.client.Write(ctx, body)
	return err
}

func parseKVResponseResult(result [][]byte, reqType string) (map[string]string, error) {
	if len(result)%2 != 0 {
		return nil, fmt.Errorf("invalid %s response: expected even number of result entries, got %d", reqType, len(result))
	}

	parsed := make(map[string]string, len(result)/2)
	for i := 0; i < len(result); i += 2 {
		parsed[string(result[i])] = string(result[i+1])
	}

	return parsed, nil
}
