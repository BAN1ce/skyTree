package utils

import "github.com/kataras/go-events"

func WithEventPrefix(prefix, s string) events.EventName {
	return events.EventName(prefix + "." + s)
}

var ()
