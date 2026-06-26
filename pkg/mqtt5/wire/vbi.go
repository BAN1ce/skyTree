package wire

// minimalVBIBytes returns the minimal MQTT Variable Byte Integer width for value.
func minimalVBIBytes(value int) int {
	switch {
	case value < 128:
		return 1
	case value < 16384:
		return 2
	case value < 2097152:
		return 3
	default:
		return 4
	}
}
