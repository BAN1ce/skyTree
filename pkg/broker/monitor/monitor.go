package monitor

import "time"

type ClientKeepAliveMonitor interface {
	SetClientAliveTime(clientID string, t *time.Time) error
}
