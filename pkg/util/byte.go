package util

import "strconv"

func Int32ToByte(i int32) []byte {
	return []byte(strconv.FormatInt(int64(i), 10))
}

func ByteToInt32(b []byte) int32 {
	i, _ := strconv.Atoi(string(b))
	return int32(i)
}
