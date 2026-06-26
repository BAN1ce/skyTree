package cluster

import (
	"math"
	"testing"
)

func Test_float64ToString(t *testing.T) {
	var (
		floatMax = math.MaxFloat64
	)

	if stringToFloat64(float64ToString(floatMax)) == floatMax {
		t.Log("float64ToString success")
	} else {
		t.Error("float64ToString failed")
	}
}
