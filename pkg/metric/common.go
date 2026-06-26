package metric

import (
	"context"
	"errors"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
)

const unknownLabel = "unknown"

var defaultDurationBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.3, 0.5, 1, 3, 5, 10}
var defaultPayloadBuckets = []float64{64, 256, 1024, 4096, 16 * 1024, 64 * 1024, 256 * 1024, 1024 * 1024}

func durationSeconds(duration time.Duration) float64 {
	if duration < 0 {
		return 0
	}
	return duration.Seconds()
}

func bytesValue(size int) float64 {
	if size < 0 {
		return 0
	}
	return float64(size)
}

func uint64Label(v uint64) string {
	return strconv.FormatUint(v, 10)
}

func normalizeResult(result string) string {
	switch result {
	case "success", "error", "timeout", "canceled", "failed", "duplicate", "negative":
		return result
	default:
		return "error"
	}
}

func resultFromError(err error) string {
	if err == nil {
		return "success"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "error"
}

func resultFromGRPCCode(code codes.Code, err error) string {
	if err != nil {
		return resultFromError(err)
	}
	if code == codes.OK {
		return "success"
	}
	return "error"
}
