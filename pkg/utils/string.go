package utils

import (
	"fmt"
	"github.com/BAN1ce/skyTree/logger"
)

func GenerateVariableData(str []byte) []byte {
	strLen := len(str)
	if strLen > 65535 {
		logger.Logger.Error("MQTT string length exceeds maximum allowed value")
	}
	buf := make([]byte, 0, strLen+2)
	if strLen > 127 {
		buf = append(buf, byte((strLen>>8)|0x80), byte(strLen&0xFF))
	} else {
		buf = append(buf, byte(strLen))
	}
	buf = append(buf, []byte(str)...)

	return buf
}

func ParseVariableData(data []byte) ([]byte, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("MQTT string is too short")
	}

	strLen := uint16(data[0])
	if strLen&0x80 == 0x80 {
		if len(data) < 2 {
			return nil, fmt.Errorf("MQTT string is too short")
		}
		strLen = ((strLen & 0x7F) << 8) | uint16(data[1])
		offset := 2
		if len(data) < int(strLen)+offset {
			return nil, fmt.Errorf("MQTT string length exceeds available data")
		}
		return data[offset : offset+int(strLen)], nil
	} else {
		// 长度字段占用一个字节
		if len(data) < int(strLen)+1 {
			return nil, fmt.Errorf("MQTT string length exceeds available data")
		}
		return data[1 : 1+strLen], nil
	}
}
