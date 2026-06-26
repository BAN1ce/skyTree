package cluster

import (
	"strconv"

	"github.com/BAN1ce/skyTree/logger"
)

func stringToFloat64(s string) float64 {
	result, err := strconv.ParseFloat(s, 10)
	if err != nil {
		logger.Logger.Error().Str("input", s).Err(err).Msg("string to float64 error")
	}
	return result
}

func float64ToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
