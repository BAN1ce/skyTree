package utils

import "time"

func NextAliveTime(keeAliveTime int64) *time.Time {
	tmp := float64(keeAliveTime) * 1.5
	keeAliveTime = int64(tmp)
	t := time.Now().Add(time.Duration(keeAliveTime) * time.Second)
	return &t
}
