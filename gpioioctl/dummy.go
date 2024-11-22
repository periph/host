// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
//
// Create a dummy chip for testing an non-Linux os.

package gpioioctl

import (
	"log"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

func makeDummyChip() {
	/*
	   During pipeline builds, GPIOChips may not be available, or
	   it may build on another OS. In that case, mock in enough
	   for a test to pass.
	*/

	line := GPIOLine{
		number:    0,
		name:      "DummyGPIOLine",
		consumer:  "",
		edge:      gpio.NoEdge,
		pull:      gpio.PullNoChange,
		direction: LineDirNotSet,
	}

	chip := GPIOChip{name: "DummyGPIOChip",
		path:      "/dev/gpiochipdummy",
		label:     "Dummy GPIOChip for Testing Purposes",
		lineCount: 1,
		lines:     []*GPIOLine{&line},
	}
	Chips = append(Chips, &chip)
	if err := gpioreg.Register(&line); err != nil {
		nameStr := chip.Name()
		lineStr := line.String()
		log.Println("chip", nameStr, " gpioreg.Register(line) ", lineStr, " returned ", err)
	}
}
