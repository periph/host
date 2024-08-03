package allwinner

import (
	"testing"
)

func Test_getH6SerializedPinSpecs(t *testing.T) {
	if pins, err := getH6SerializedPinSpecs(); err != nil {
		t.Error(err)
	} else if n := len(pins); n != 74 {
		t.Errorf("Expected %d to equal %d", n, 74)
	}
}
