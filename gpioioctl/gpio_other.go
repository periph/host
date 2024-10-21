//go:build !linux

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
//
// Create a dummy chip because ioctl is only supported on Linux

package gpioioctl

import (
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

func init() {
	if len(Chips) == 0 {
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
		Chips = append(Chips, &chip)
		if err = gpioreg.Register(&line); err != nil {
			nameStr := chip.Name()
			lineStr := line.String()
			log.Println("chip", nameStr, " gpioreg.Register(line) ", lineStr, " returned ", err)
		}
	}
}
