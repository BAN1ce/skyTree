package config

import "time"

// DeliveryRunner controls the online client delivery runner behavior.
// This config makes the runner event-driven at scale and avoids per-client polling.
type DeliveryRunner struct {
	// WakeMaxWait is the maximum time to wait for wake signal before doing a single fallback ReadTasks probe.
	WakeMaxWait time.Duration `yaml:"wake_max_wait" env:"DELIVERY_WAKE_MAX_WAIT" env-default:"3m"`

	// WakeReadRetryMaxAttempts controls how many retries to perform ONLY after a wake-triggered ReadTasks fails.
	WakeReadRetryMaxAttempts int `yaml:"wake_read_retry_max_attempts" env:"DELIVERY_WAKE_READ_RETRY_MAX_ATTEMPTS" env-default:"6"`

	// WakeReadRetryBackoffInitial is the initial backoff used for wake-triggered read failures.
	WakeReadRetryBackoffInitial time.Duration `yaml:"wake_read_retry_backoff_initial" env:"DELIVERY_WAKE_READ_RETRY_BACKOFF_INITIAL" env-default:"50ms"`

	// WakeReadRetryBackoffMax is the max backoff used for wake-triggered read failures.
	WakeReadRetryBackoffMax time.Duration `yaml:"wake_read_retry_backoff_max" env:"DELIVERY_WAKE_READ_RETRY_BACKOFF_MAX" env-default:"1s"`

	// InflightWaitTick is the safety tick used while the client has QoS1/QoS2 inflight messages.
	InflightWaitTick time.Duration `yaml:"inflight_wait_tick" env:"DELIVERY_INFLIGHT_WAIT_TICK" env-default:"1s"`

	// InflightRetransmitInterval controls how often to retransmit downlink QoS1/QoS2 inflight messages.
	InflightRetransmitInterval time.Duration `yaml:"inflight_retransmit_interval" env:"DELIVERY_INFLIGHT_RETRANSMIT_INTERVAL" env-default:"5s"`

	// InflightMaxRetries is the maximum number of retransmission attempts before disconnecting the client.
	InflightMaxRetries int `yaml:"inflight_max_retries" env:"DELIVERY_INFLIGHT_MAX_RETRIES" env-default:"5"`

	// InflightMaxAge is the maximum age since first send for an inflight message before disconnecting the client.
	InflightMaxAge time.Duration `yaml:"inflight_max_age" env:"DELIVERY_INFLIGHT_MAX_AGE" env-default:"2m"`

	// OutgoingCommitInterval is the maximum time between two in-flight commits of downlink
	// delivery progress (QoS1 replay cursor + QoS2/retained unfinished) to the session store.
	// It bounds the time-based replay window after an unexpected crash. 0 disables time-based commits.
	OutgoingCommitInterval time.Duration `yaml:"outgoing_commit_interval" env:"DELIVERY_OUTGOING_COMMIT_INTERVAL" env-default:"5s"`

	// OutgoingCommitMaxAcks is the number of downlink terminal ACKs (QoS1 PUBACK / QoS2 PUBCOMP)
	// after which delivery progress is committed to the session store, regardless of the timer.
	// It bounds the count-based replay window for high-throughput clients. 0 disables count-based commits.
	OutgoingCommitMaxAcks int `yaml:"outgoing_commit_max_acks" env:"DELIVERY_OUTGOING_COMMIT_MAX_ACKS" env-default:"500"`
}

// DefaultDeliveryRunner returns default config values for startup-less contexts (tests/utilities).
func DefaultDeliveryRunner() DeliveryRunner {
	return DeliveryRunner{
		WakeMaxWait:                 3 * time.Minute,
		WakeReadRetryMaxAttempts:    6,
		WakeReadRetryBackoffInitial: 50 * time.Millisecond,
		WakeReadRetryBackoffMax:     1 * time.Second,
		InflightWaitTick:            1 * time.Second,
		InflightRetransmitInterval:  5 * time.Second,
		InflightMaxRetries:          5,
		InflightMaxAge:              2 * time.Minute,
		OutgoingCommitInterval:      5 * time.Second,
		OutgoingCommitMaxAcks:       500,
	}
}
