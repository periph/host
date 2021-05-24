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

// dbusSync is the handler of a synchronous bitbang on DBus.
//
// More details at:
// http://www.ftdichip.com/Support/Documents/AppNotes/AN_232R-01_Bit_Bang_Mode_Available_For_FT232R_and_Ft245R.pdf
type dbusSync interface {
	dbusSyncGPIOFunc(n int) string
	dbusSyncGPIOIn(n int) error
	dbusSyncGPIORead(n int) gpio.Level
	dbusSyncGPIOOut(n int, l gpio.Level) error
}

// dbusPinSync represents a GPIO on a synchronous bitbang DBus.
//
// It is immutable and stateless.
type dbusPinSync struct {
	n   string
	num int
	bus dbusSync
}

// String implements conn.Resource.
func (s *dbusPinSync) String() string {
	return s.n
}

// Halt implements conn.Resource.
func (s *dbusPinSync) Halt() error {
	return nil
}

// Name implements pin.Pin.
func (s *dbusPinSync) Name() string {
	return s.n
}

// Number implements pin.Pin.
func (s *dbusPinSync) Number() int {
	return s.num
}

// Function implements pin.Pin.
func (s *dbusPinSync) Function() string {
	return s.bus.dbusSyncGPIOFunc(s.num)
}

// In implements gpio.PinIn.
func (s *dbusPinSync) In(pull gpio.Pull, e gpio.Edge) error {
	if e != gpio.NoEdge {
		// We could support it on D5.
		return errors.New("d2xx: edge triggering is not supported")
	}
	if pull != gpio.PullUp && pull != gpio.PullNoChange {
		// EEPROM has a PullDownEnable flag.
		return errors.New("d2xx: pull is not supported")
	}
	return s.bus.dbusSyncGPIOIn(s.num)
}

// Read implements gpio.PinIn.
func (s *dbusPinSync) Read() gpio.Level {
	return s.bus.dbusSyncGPIORead(s.num)
}

// WaitForEdge implements gpio.PinIn.
func (s *dbusPinSync) WaitForEdge(t time.Duration) bool {
	return false
}

// DefaultPull implements gpio.PinIn.
func (s *dbusPinSync) DefaultPull() gpio.Pull {
	// 200kΩ
	// http://www.ftdichip.com/Support/Documents/DataSheets/ICs/DS_FT232R.pdf
	// p. 24
	return gpio.PullUp
}

// Pull implements gpio.PinIn.
func (s *dbusPinSync) Pull() gpio.Pull {
	return gpio.PullUp
}

// Out implements gpio.PinOut.
func (s *dbusPinSync) Out(l gpio.Level) error {
	return s.bus.dbusSyncGPIOOut(s.num, l)
}

// PWM implements gpio.PinOut.
func (s *dbusPinSync) PWM(d gpio.Duty, f physic.Frequency) error {
	return errors.New("d2xx: not implemented")
}

/*
func (s *dbusPinSync) Drive() physic.ElectricCurrent {
	// optionally 3
	//return s.bus.ee.DDriveCurrent * physic.MilliAmpere
	return physic.MilliAmpere
}

func (s *dbusPinSync) SlewLimit() bool {
	//return s.bus.ee.DSlowSlew
	return false
}

func (s *dbusPinSync) Hysteresis() bool {
	//return s.bus.ee.DSchmittInput
	return true
}
*/

//

// cBusGPIO is the handler of a CBus bitbang bus.
//
// This is an asynchronous mode.
//
// More details at:
// http://www.ftdichip.com/Support/Knowledgebase/index.html?cbusbitbangmode.htm
type cBusGPIO interface {
	cBusGPIOFunc(n int) string
	cBusGPIOIn(n int) error
	cBusGPIORead(n int) gpio.Level
	cBusGPIOOut(n int, l gpio.Level) error
}

// cbusPin represents a GPIO on a CBus bitbang bus.
//
// It is immutable and stateless.
type cbusPin struct {
	n   string
	num int
	p   gpio.Pull
	bus cBusGPIO
}

// String implements conn.Resource.
func (c *cbusPin) String() string {
	return c.n
}

// Halt implements conn.Resource.
func (c *cbusPin) Halt() error {
	return nil
}

// Name implements pin.Pin.
func (c *cbusPin) Name() string {
	return c.n
}

// Number implements pin.Pin.
func (c *cbusPin) Number() int {
	return c.num
}

// Function implements pin.Pin.
func (c *cbusPin) Function() string {
	return c.bus.cBusGPIOFunc(c.num)
}

// In implements gpio.PinIn.
func (c *cbusPin) In(pull gpio.Pull, e gpio.Edge) error {
	if e != gpio.NoEdge {
		// We could support it on D5.
		return errors.New("d2xx: edge triggering is not supported")
	}
	if pull != c.p && pull != gpio.PullNoChange {
		// EEPROM has a PullDownEnable flag.
		return errors.New("d2xx: pull is not supported")
	}
	return c.bus.cBusGPIOIn(c.num)
}

// Read implements gpio.PinIn.
func (c *cbusPin) Read() gpio.Level {
	return c.bus.cBusGPIORead(c.num)
}

// WaitForEdge implements gpio.PinIn.
func (c *cbusPin) WaitForEdge(t time.Duration) bool {
	return false
}

// DefaultPull implements gpio.PinIn.
func (c *cbusPin) DefaultPull() gpio.Pull {
	// 200kΩ
	// http://www.ftdichip.com/Support/Documents/DataSheets/ICs/DS_FT232R.pdf
	// p. 24
	return c.p
}

// Pull implements gpio.PinIn.
func (c *cbusPin) Pull() gpio.Pull {
	return c.p
}

// Out implements gpio.PinOut.
func (c *cbusPin) Out(l gpio.Level) error {
	return c.bus.cBusGPIOOut(c.num, l)
}

// PWM implements gpio.PinOut.
func (c *cbusPin) PWM(d gpio.Duty, f physic.Frequency) error {
	return errors.New("d2xx: not implemented")
}

/*
func (c *cbusPin) Drive() physic.ElectricCurrent {
	// optionally 3
	//return c.bus.ee.CDriveCurrent * physic.MilliAmpere
	return physic.MilliAmpere
}

func (c *cbusPin) SlewLimit() bool {
	//return c.bus.ee.CSlowSlew
	return false
}

func (c *cbusPin) Hysteresis() bool {
	//return c.bus.ee.CSchmittInput
	return true
}
*/

var _ gpio.PinIO = &dbusPinSync{}
var _ gpio.PinIO = &cbusPin{}
