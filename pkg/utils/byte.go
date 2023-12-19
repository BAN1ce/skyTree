package utils

import (
	"bytes"
	"strconv"
)

func Int32ToByte(i int32) []byte {
	return []byte(strconv.FormatInt(int64(i), 10))
}

func ByteToInt32(b []byte) int32 {
	i, _ := strconv.Atoi(string(b))
	return int32(i)
}

func WriteUint32(u uint32, b *bytes.Buffer) error {
	if err := b.WriteByte(byte(u >> 24)); err != nil {
		return err
	}
	if err := b.WriteByte(byte(u >> 16)); err != nil {
		return err
	}
	if err := b.WriteByte(byte(u >> 8)); err != nil {
		return err
	}
	return b.WriteByte(byte(u))
}

func ReadUint32(b *bytes.Buffer) (uint32, error) {
	b1, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b2, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b3, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b4, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	return (uint32(b1) << 24) | (uint32(b2) << 16) | (uint32(b3) << 8) | uint32(b4), nil
}

func WriteUint64(u uint64, b *bytes.Buffer) error {
	if err := WriteUint32(uint32(u>>32), b); err != nil {
		return err
	}
	return WriteUint32(uint32(u), b)
}

func ReadUint64(b *bytes.Buffer) (uint64, error) {
	b1, err := ReadUint32(b)
	if err != nil {
		return 0, err
	}
	b2, err := ReadUint32(b)
	if err != nil {
		return 0, err
	}
	return (uint64(b1) << 32) | uint64(b2), nil
}
