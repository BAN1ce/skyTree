package config

import "time"

type TimeWheel struct {
	Interval time.Duration
	SlotNum  int
}

func (t TimeWheel) GetInterval() time.Duration {
	return t.Interval
}

func (t TimeWheel) GetSlotNum() int {
	return t.SlotNum
}

func GetTimeWheel() TimeWheel {
	return TimeWheel{
		Interval: 1000 * time.Millisecond,
		SlotNum:  1000,
	}
}
