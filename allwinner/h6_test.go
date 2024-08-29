// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package allwinner

import (
	"testing"
)

func TestGetH6SerializedPinSpecs(t *testing.T) {
	if pins, err := getH6SerializedPinSpecs(); err != nil {
		t.Error(err)
	} else if n := len(pins); n != 72 {
		t.Errorf("Expected %d to equal %d", n, 72)
	}
}

func TestGetH6SerializedPinSpecs_areRegistered(t *testing.T) {
	pins, err := getH6SerializedPinSpecs()
	if err != nil {
		t.Fatal(err)
	}
	for _, pinSpec := range pins {
		if _, ok := cpupins[pinSpec.Name]; !ok {
			t.Error(pinSpec.Name)
		}
	}
}
