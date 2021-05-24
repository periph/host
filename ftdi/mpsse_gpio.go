// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ftdi

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
)

// gpiosMPSSE is a slice of 8 GPIO pins driven via MPSSE.
//
// This permits keeping a cache.
type gpiosMPSSE struct {
	// Immutable.
	h    *handle
	cbus bool // false if D bus
	pins [8]gpioMPSSE

	// Cache of values
	direction byte
	value     byte
}

func (g *gpiosMPSSE) init(name string) {
	s := "D"
	if g.cbus {
		s = "C"
	}
	// Configure pulls; pull ups are 75kΩ.
	// http://www.ftdichip.com/Support/Documents/AppNotes/AN_184%20FTDI%20Device%20Input%20Output%20Pin%20States.pdf
	// has a good table.
	// D0, D2 and D4 go in high impedance before going into pull up.
	// TODO(maruel): The pull on CBus depends on EEPROM!
	for i := range g.pins {
		g.pins[i].a = g
		g.pins[i].n = name + "." + s + strconv.Itoa(i)
		g.pins[i].num = i
		g.pins[i].dp = gpio.PullUp
	}
	if g.cbus {
		// That's just the default EEPROM value.
		g.pins[7].dp = gpio.PullDown
	}
}

func (g *gpiosMPSSE) in(n int) error {
	if g.h == nil {
		return errors.New("d2xx: device not open")
	}
	g.direction = g.direction & ^(1 << uint(n))
	if g.cbus {
		return g.h.MPSSECBus(g.direction, g.value)
	}
	return g.h.MPSSEDBus(g.direction, g.value)
}

func (g *gpiosMPSSE) read() (byte, error) {
	if g.h == nil {
		return 0, errors.New("d2xx: device not open")
	}
	var err error
	if g.cbus {
		g.value, err = g.h.MPSSECBusRead()
	} else {
		g.value, err = g.h.MPSSEDBusRead()
	}
	return g.value, err
}

func (g *gpiosMPSSE) out(n int, l gpio.Level) error {
	if g.h == nil {
		return errors.New("d2xx: device not open")
	}
	g.direction = g.direction | (1 << uint(n))
	if l {
		g.value |= 1 << uint(n)
	} else {
		g.value &^= 1 << uint(n)
	}
	if g.cbus {
		return g.h.MPSSECBus(g.direction, g.value)
	}
	return g.h.MPSSEDBus(g.direction, g.value)
}

//

// gpioMPSSE is a GPIO pin on a FTDI device driven via MPSSE.
//
// gpioMPSSE implements gpio.PinIO.
//
// It is immutable and stateless.
type gpioMPSSE struct {
	a   *gpiosMPSSE
	n   string
	num int
	dp  gpio.Pull
}

// String implements pin.Pin.
func (g *gpioMPSSE) String() string {
	return g.n
}

// Name implements pin.Pin.
func (g *gpioMPSSE) Name() string {
	return g.n
}

// Number implements pin.Pin.
func (g *gpioMPSSE) Number() int {
	return g.num
}

// Function implements pin.Pin.
func (g *gpioMPSSE) Function() string {
	s := "Out/"
	m := byte(1 << uint(g.num))
	if g.a.direction&m == 0 {
		s = "In/"
		_, _ = g.a.read()
	}
	return s + gpio.Level(g.a.value&m != 0).String()
}

// Halt implements gpio.PinIO.
func (g *gpioMPSSE) Halt() error {
	return nil
}

// In implements gpio.PinIn.
func (g *gpioMPSSE) In(pull gpio.Pull, e gpio.Edge) error {
	if e != gpio.NoEdge {
		// We could support it on D5.
		return errors.New("d2xx: edge triggering is not supported")
	}
	if pull != g.dp && pull != gpio.PullNoChange {
		// TODO(maruel): This needs to be redone:
		// - EEPROM values FT232hCBusTristatePullUp and FT232hCBusPwrEnable can be
		//   used to control individual CBus pins.
		// - dataTristate enables gpio.Float when set to output High, but I don't
		//   know if it will enable reading the value (?). This needs to be
		//   confirmed.
		return fmt.Errorf("d2xx: pull %s is not supported; try %s", pull, g.dp)
	}
	return g.a.in(g.num)
}

// Read implements gpio.PinIn.
func (g *gpioMPSSE) Read() gpio.Level {
	v, _ := g.a.read()
	return gpio.Level(v&(1<<uint(g.num)) != 0)
}

// WaitForEdge implements gpio.PinIn.
func (g *gpioMPSSE) WaitForEdge(t time.Duration) bool {
	return false
}

// DefaultPull implements gpio.PinIn.
func (g *gpioMPSSE) DefaultPull() gpio.Pull {
	return g.dp
}

// Pull implements gpio.PinIn. The resistor is 75kΩ.
func (g *gpioMPSSE) Pull() gpio.Pull {
	// See In() for the challenges.
	return g.dp
}

// Out implements gpio.PinOut.
func (g *gpioMPSSE) Out(l gpio.Level) error {
	return g.a.out(g.num, l)
}

// PWM implements gpio.PinOut.
func (g *gpioMPSSE) PWM(d gpio.Duty, f physic.Frequency) error {
	return errors.New("d2xx: not implemented")
}

/*
func (g *gpioMPSSE) Drive() physic.ElectricCurrent {
	//return g.a.ee.CDriveCurrent * physic.MilliAmpere
	return 2 * physic.MilliAmpere
}

func (g *gpioMPSSE) SlewLimit() bool {
	//return g.a.ee.CSlowSlew
	return false
}

func (g *gpioMPSSE) Hysteresis() bool {
	//return g.a.ee.DSchmittInput
	return true
}
*/

var _ gpio.PinIO = &gpioMPSSE{}
