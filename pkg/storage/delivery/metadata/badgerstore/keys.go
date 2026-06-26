package badgerstore

import (
	"bytes"
	"encoding/binary"
)

var (
	prefixTask        = []byte("dq:t:")
	prefixCursor      = []byte("dq:c:")
	prefixShareTask   = []byte("sq:t:")
	prefixShareCursor = []byte("sq:c:")
)

func taskKeyPrefix(clientID string) []byte {
	// dq:t:{clientID}\x00
	b := make([]byte, 0, len(prefixTask)+len(clientID)+1)
	b = append(b, prefixTask...)
	b = append(b, []byte(clientID)...)
	b = append(b, 0)
	return b
}

func taskKey(clientID string, tsUnixNano int64, taskIDBytes [16]byte) []byte {
	p := taskKeyPrefix(clientID)
	key := make([]byte, 0, len(p)+8+16)
	key = append(key, p...)
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(tsUnixNano))
	key = append(key, ts[:]...)
	key = append(key, taskIDBytes[:]...)
	return key
}

func cursorKey(clientID string) []byte {
	// dq:c:{clientID}
	b := make([]byte, 0, len(prefixCursor)+len(clientID))
	b = append(b, prefixCursor...)
	b = append(b, []byte(clientID)...)
	return b
}

func shareTaskKeyPrefix(shareGroup string) []byte {
	// sq:t:{shareGroup}\x00
	b := make([]byte, 0, len(prefixShareTask)+len(shareGroup)+1)
	b = append(b, prefixShareTask...)
	b = append(b, []byte(shareGroup)...)
	b = append(b, 0)
	return b
}

func shareTaskKey(shareGroup string, tsUnixNano int64, taskIDBytes [16]byte) []byte {
	p := shareTaskKeyPrefix(shareGroup)
	key := make([]byte, 0, len(p)+8+16)
	key = append(key, p...)
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(tsUnixNano))
	key = append(key, ts[:]...)
	key = append(key, taskIDBytes[:]...)
	return key
}

func shareCursorKey(shareGroup string) []byte {
	// sq:c:{shareGroup}
	b := make([]byte, 0, len(prefixShareCursor)+len(shareGroup))
	b = append(b, prefixShareCursor...)
	b = append(b, []byte(shareGroup)...)
	return b
}

func parseTaskKey(key []byte, expectClientPrefix []byte) (tsUnixNano int64, taskIDBytes [16]byte, ok bool) {
	// key = expectClientPrefix + ts(8) + taskID(16)
	if !bytes.HasPrefix(key, expectClientPrefix) {
		return 0, taskIDBytes, false
	}
	rest := key[len(expectClientPrefix):]
	if len(rest) < 8+16 {
		return 0, taskIDBytes, false
	}
	tsUnixNano = int64(binary.BigEndian.Uint64(rest[:8]))
	copy(taskIDBytes[:], rest[8:8+16])
	return tsUnixNano, taskIDBytes, true
}

func parseShareTaskKey(key []byte, expectGroupPrefix []byte) (tsUnixNano int64, taskIDBytes [16]byte, ok bool) {
	// key = expectGroupPrefix + ts(8) + taskID(16)
	if !bytes.HasPrefix(key, expectGroupPrefix) {
		return 0, taskIDBytes, false
	}
	rest := key[len(expectGroupPrefix):]
	if len(rest) < 8+16 {
		return 0, taskIDBytes, false
	}
	tsUnixNano = int64(binary.BigEndian.Uint64(rest[:8]))
	copy(taskIDBytes[:], rest[8:8+16])
	return tsUnixNano, taskIDBytes, true
}
