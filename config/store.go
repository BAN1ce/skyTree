package config

import "time"

type Store struct {
	ReadTimeout time.Duration
}

func GetStore() Store {
	return Store{
		ReadTimeout: 3000 * time.Millisecond,
	}
}
