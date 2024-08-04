package allwinner

import (
	"testing"
)

func Test_getH6SerializedPinSpecs(t *testing.T) {
	if pins, err := getH6SerializedPinSpecs(); err != nil {
		t.Error(err)
	} else if n := len(pins); n != 72 {
		t.Errorf("Expected %d to equal %d", n, 72)
	}
}

func Test_getH6SerializedPinSpecs_areRegistered(t *testing.T) {
	if pins, err := getH6SerializedPinSpecs(); err != nil {
		t.Error(err)
	} else {
		for _, pinSpec := range pins {
			if _, ok := cpupins[pinSpec.Name]; !ok {
				t.Error(pinSpec.Name)
			}
		}
	}
}
