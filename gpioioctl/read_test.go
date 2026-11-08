// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gpioioctl

import (
	"testing"

	"periph.io/x/conn/v3/gpio"
)

func init() {
	if len(Chips) == 0 {
		makeDummyChip()
	}
}

// TestReadOutputPinPreservesDirection verifies that calling Read() on a pin
// configured as output does not reconfigure it as input. This is a regression
// test for https://github.com/periph/host/issues/75.
//
// The bug: Read() called In(gpio.PullUp, gpio.NoEdge) on output pins,
// silently switching them from output to input with pull-up enabled. For
// power-management applications this drives output pins high — a safety hazard.
func TestReadOutputPinPreservesDirection(t *testing.T) {
	line := Chips[0].Lines()[0]

	// Configure as output. On non-Linux this will fail at the ioctl level,
	// but the direction field is set before the ioctl call.
	line.direction = LineOutput
	line.pull = gpio.PullNoChange
	line.edge = gpio.NoEdge

	// Read the pin value. This should NOT change the direction.
	_ = line.Read()

	if line.direction != LineOutput {
		t.Errorf("Read() changed direction from Output to %s; want Output preserved",
			DirectionLabels[line.direction])
	}
	if line.pull != gpio.PullNoChange {
		t.Errorf("Read() changed pull from PullNoChange to %s; want PullNoChange preserved",
			PullLabels[line.pull])
	}
}

// TestReadOutputPinDoesNotCallIn verifies that Read() on an output pin reads
// the driven value directly without calling In() to reconfigure the pin.
func TestReadOutputPinDoesNotCallIn(t *testing.T) {
	line := Chips[0].Lines()[0]

	line.direction = LineOutput
	line.pull = gpio.PullNoChange
	line.edge = gpio.NoEdge

	// After Read(), direction must still be output.
	_ = line.Read()

	if line.direction == LineInput {
		t.Fatal("Read() reconfigured output pin as input — this drives output pins high via pull-up")
	}
}
