// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Emulate independent GPIOs.

package ftdi

import (
	"errors"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
)

// invalidPin is a non-working (not implemented) pin on a FTDI device.
//
// invalidPin implements gpio.PinIO.
type invalidPin struct {
	n   string
	num int
}

// String implements pin.Pin.
func (p *invalidPin) String() string {
	return p.n
}

// Name implements pin.Pin.
func (p *invalidPin) Name() string {
	return p.n
}

// Number implements pin.Pin.
func (p *invalidPin) Number() int {
	return p.num
}

// Function implements pin.Pin.
func (p *invalidPin) Function() string {
	return "N/A"
}

// Halt implements gpio.PinIO.
func (p *invalidPin) Halt() error {
	return nil
}

// In implements gpio.PinIn.
func (p *invalidPin) In(pull gpio.Pull, e gpio.Edge) error {
	return errors.New("d2xx: to be implemented")
}

// Read implements gpio.PinIn.
func (p *invalidPin) Read() gpio.Level {
	return gpio.Low
}

// WaitForEdge implements gpio.PinIn.
func (p *invalidPin) WaitForEdge(t time.Duration) bool {
	return false
}

// Pull implements gpio.PinIn.
func (p *invalidPin) Pull() gpio.Pull {
	return gpio.PullNoChange
}

// DefaultPull implements gpio.PinIn.
func (p *invalidPin) DefaultPull() gpio.Pull {
	return gpio.PullNoChange
}

// Out implements gpio.PinOut.
func (p *invalidPin) Out(l gpio.Level) error {
	return errors.New("d2xx: to be implemented")
}

// PWM implements gpio.PinOut.
func (p *invalidPin) PWM(d gpio.Duty, f physic.Frequency) error {
	return errors.New("d2xx: to be implemented")
}

var _ gpio.PinIO = &invalidPin{}
