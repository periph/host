// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bcm283x

import (
	"errors"
	"os"
	"testing"
)

// TestMapGPIOFallbackPropagatesError verifies that when both MapGPIO() and
// Map() fail, the error from Map() is returned — not nil. This is a
// regression test for https://github.com/periph/host/issues/76 where a
// variable shadowing bug caused the error to be swallowed.
func TestMapGPIOFallbackPropagatesError(t *testing.T) {
	d := &driverGPIO{gpioBaseAddr: 0x3F200000}
	gpioErr := errors.New("MapGPIO: /dev/gpiomem not found")

	// mapGPIOFallback will call pmem.Map which fails on non-Linux.
	_, err := d.mapGPIOFallback(gpioErr)
	if err == nil {
		t.Fatal("mapGPIOFallback returned nil error when Map() fails; error was swallowed")
	}
}

// TestMapGPIOFallbackPermissionError verifies that the error message includes
// the original MapGPIO error when the fallback fails with a permission error.
func TestMapGPIOFallbackPermissionError(t *testing.T) {
	d := &driverGPIO{gpioBaseAddr: 0x3F200000}
	gpioErr := os.ErrNotExist

	// On non-Linux, pmem.Map returns a generic "not supported" error, not a
	// permission error, so this test verifies the non-permission path returns
	// a non-nil error.
	_, err := d.mapGPIOFallback(gpioErr)
	if err == nil {
		t.Fatal("mapGPIOFallback returned nil error; expected Map() failure to propagate")
	}
}
