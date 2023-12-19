package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ClientConnect = promauto.NewCounter(prometheus.CounterOpts{
		Name: "client_connect",
	})

	ClientDisconnect = promauto.NewCounter(prometheus.CounterOpts{
		Name: "client_disconnect",
	})

	ClientOnline = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "client_online",
	})
)
