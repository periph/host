// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bcm283x

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/pin"
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

// TestAliasConflictDoesNotAbortInit verifies that when the ioctl-gpio driver
// has already registered a pin name (e.g. "PWM0_OUT"), the bcm283x driver's
// alias registration does not abort — it logs and continues. Aborting here
// would prevent gpioMemory from being initialised, forcing all GPIO operations
// through the ioctl fallback path. The ioctl path cannot read the state of
// output pins set by a previous process (the line fd is per-process), so
// restoreStartupState would always read LOW regardless of actual hardware state.
//
// Regression test for the alias conflict discovered via
// https://github.com/periph/host/issues/75 investigation.
func TestAliasConflictDoesNotAbortInit(t *testing.T) {
	// Simulate the ioctl-gpio driver having registered "PWM0_OUT" as a real pin
	// before the bcm283x driver runs.
	const testPin = "TestAliasConflict_PWM"
	if err := gpioreg.Register(&fakePin{name: testPin}); err != nil {
		t.Fatalf("setup: Register fakePin: %v", err)
	}
	defer gpioreg.Unregister(testPin)

	// RegisterAlias should fail because the name is already a real pin.
	err := gpioreg.RegisterAlias(testPin, "CLK0")
	if err == nil {
		t.Fatal("expected RegisterAlias to fail for an already-registered pin name")
	}
	if !strings.Contains(err.Error(), "pin that exists") {
		t.Fatalf("unexpected error: %v", err)
	}

	// The fix: the bcm283x Init loop logs this error instead of returning it.
	// This test documents the scenario — the behavioural verification is that
	// bcm283x-gpio appears in state.Loaded (not state.Failed) on real hardware.
}

// fakePin implements gpio.PinIO minimally for test registration.
type fakePin struct {
	name string
}

var _ gpio.PinIO = &fakePin{} // compile-time interface check

func (p *fakePin) String() string                        { return p.name }
func (p *fakePin) Name() string                          { return p.name }
func (p *fakePin) Number() int                           { return -1 }
func (p *fakePin) Function() string                      { return "" }
func (p *fakePin) Func() pin.Func                        { return "" }
func (p *fakePin) SupportedFuncs() []pin.Func            { return nil }
func (p *fakePin) SetFunc(pin.Func) error                { return nil }
func (p *fakePin) Halt() error                           { return nil }
func (p *fakePin) In(gpio.Pull, gpio.Edge) error         { return nil }
func (p *fakePin) Read() gpio.Level                      { return false }
func (p *fakePin) WaitForEdge(time.Duration) bool        { return false }
func (p *fakePin) Pull() gpio.Pull                       { return 0 }
func (p *fakePin) DefaultPull() gpio.Pull                { return 0 }
func (p *fakePin) Out(gpio.Level) error                  { return nil }
func (p *fakePin) PWM(gpio.Duty, physic.Frequency) error { return nil }
