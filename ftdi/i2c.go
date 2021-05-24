// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// This functionality requires MPSSE.
//
// Interfacing I²C:
// http://www.ftdichip.com/Support/Documents/AppNotes/AN_113_FTDI_Hi_Speed_USB_To_I2C_Example.pdf
//
// Implementation based on
// http://www.ftdichip.com/Support/Documents/AppNotes/AN_255_USB%20to%20I2C%20Example%20using%20the%20FT232H%20and%20FT201X%20devices.pdf
//
// Page 18: MPSSE does not automatically support clock stretching for I²C.

package ftdi

import (
	"context"
	"errors"
	"fmt"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/physic"
)

const i2cSCL = 1    // D0
const i2cSDAOut = 2 // D1
const i2cSDAIn = 4  // D2

type i2cBus struct {
	f      *FT232H
	pullUp bool
}

// Close stops I²C mode, returns to high speed mode, disable tri-state.
func (d *i2cBus) Close() error {
	d.f.mu.Lock()
	err := d.stopI2C()
	d.f.mu.Unlock()
	return err
}

// Duplex implements conn.Conn.
func (d *i2cBus) Duplex() conn.Duplex {
	return conn.Half
}

func (d *i2cBus) String() string {
	return d.f.String()
}

// SetSpeed implements i2c.Bus.
func (d *i2cBus) SetSpeed(f physic.Frequency) error {
	if f > 10*physic.MegaHertz {
		return fmt.Errorf("d2xx: invalid speed %s; maximum supported clock is 10MHz", f)
	}
	if f < 100*physic.Hertz {
		return fmt.Errorf("d2xx: invalid speed %s; minimum supported clock is 100Hz; did you forget to multiply by physic.KiloHertz?", f)
	}
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	_, err := d.f.h.MPSSEClock(f * 2 / 3)
	return err
}

// Tx implements i2c.Bus.
func (d *i2cBus) Tx(addr uint16, w, r []byte) error {
	d.f.mu.Lock()
	defer d.f.mu.Unlock()
	if err := d.setI2CStart(); err != nil {
		return err
	}
	a := [1]byte{byte(addr)}
	if err := d.writeBytes(a[:]); err != nil {
		return err
	}
	if len(w) != 0 {
		if err := d.writeBytes(w); err != nil {
			return err
		}
	}
	if len(r) != 0 {
		if err := d.readBytes(r); err != nil {
			return err
		}
	}
	if err := d.setI2CStop(); err != nil {
		return err
	}
	return d.setI2CLinesIdle()
}

// SCL implements i2c.Pins.
func (d *i2cBus) SCL() gpio.PinIO {
	return d.f.D0
}

// SDA implements i2c.Pins.
func (d *i2cBus) SDA() gpio.PinIO {
	return d.f.D1
}

// setupI2C initializes the MPSSE to the state to run an I²C transaction.
//
// Defaults to 400kHz.
//
// When pullUp is true; output alternates between Out(Low) and In(PullUp).
//
// when pullUp is false; pins are set in Tristate so Out(High) becomes float
// instead of drive High. Low still drives low. That's called open collector.
func (d *i2cBus) setupI2C(pullUp bool) error {
	if pullUp {
		return errors.New("d2xx: PullUp will soon be implemented")
	}
	// TODO(maruel): We could set these only *during* the I²C operation, which
	// would make more sense.
	f := 400 * physic.KiloHertz
	clk := ((30 * physic.MegaHertz / f) - 1) * 2 / 3

	buf := [4 + 3]byte{
		clock3Phase,
		clock30MHz, byte(clk), byte(clk >> 8),
	}
	cmd := buf[:4]
	if !d.pullUp {
		// TODO(maruel): Do not mess with other GPIOs tristate.
		cmd = append(cmd, dataTristate, 7, 0)
	}
	if _, err := d.f.h.Write(cmd); err != nil {
		return err
	}
	d.f.usingI2C = true
	d.pullUp = pullUp
	return d.setI2CLinesIdle()
}

// stopI2C resets the MPSSE to a more "normal" state.
func (d *i2cBus) stopI2C() error {
	// Resets to 30MHz.
	buf := [4 + 3]byte{
		clock2Phase,
		clock30MHz, 0, 0,
	}
	cmd := buf[:4]
	if !d.pullUp {
		// TODO(maruel): Do not mess with other GPIOs tristate.
		cmd = append(cmd, dataTristate, 0, 0)
	}
	_, err := d.f.h.Write(cmd)
	d.f.usingI2C = false
	return err
}

// setI2CLinesIdle sets all D0 and D1 lines high.
//
// Does not touch D3~D7.
func (d *i2cBus) setI2CLinesIdle() error {
	const mask = 0xFF &^ (i2cSCL | i2cSDAOut | i2cSDAIn)
	// TODO(maruel): d.pullUp
	d.f.dbus.direction = d.f.dbus.direction&mask | i2cSCL | i2cSDAOut
	d.f.dbus.value = d.f.dbus.value & mask
	cmd := [...]byte{gpioSetD, d.f.dbus.value | i2cSCL | i2cSDAOut, d.f.dbus.direction}
	_, err := d.f.h.Write(cmd[:])
	return err
}

// setI2CStart starts an I²C transaction.
//
// Does not touch D3~D7.
func (d *i2cBus) setI2CStart() error {
	// TODO(maruel): d.pullUp
	dir := d.f.dbus.direction
	v := d.f.dbus.value
	// Assumes last setup was d.setI2CLinesIdle(), e.g. D0 and D1 are high, so
	// skip this.
	//
	// Runs the command 4 times as a way to delay execution.
	cmd := [...]byte{
		// SCL high, SDA low for 600ns
		gpioSetD, v | i2cSCL, dir,
		gpioSetD, v | i2cSCL, dir,
		gpioSetD, v | i2cSCL, dir,
		gpioSetD, v | i2cSCL, dir,
		// SCL low, SDA low
		gpioSetD, v, dir,
		gpioSetD, v, dir,
		gpioSetD, v, dir,
	}
	_, err := d.f.h.Write(cmd[:])
	return err
}

// setI2CStop completes an I²C transaction.
//
// Does not touch D3~D7.
func (d *i2cBus) setI2CStop() error {
	// TODO(maruel): d.pullUp
	dir := d.f.dbus.direction
	v := d.f.dbus.value
	// Runs the command 4 times as a way to delay execution.
	cmd := [...]byte{
		// SCL low, SDA low
		gpioSetD, v, dir,
		gpioSetD, v, dir,
		gpioSetD, v, dir,
		gpioSetD, v, dir,
		// SCL high, SDA low
		gpioSetD, v | i2cSCL, dir,
		gpioSetD, v | i2cSCL, dir,
		gpioSetD, v | i2cSCL, dir,
		gpioSetD, v | i2cSCL, dir,
		// SCL high, SDA high
		gpioSetD, v | i2cSCL | i2cSDAOut, dir,
		gpioSetD, v | i2cSCL | i2cSDAOut, dir,
		gpioSetD, v | i2cSCL | i2cSDAOut, dir,
		gpioSetD, v | i2cSCL | i2cSDAOut, dir,
	}
	_, err := d.f.h.Write(cmd[:])
	return err
}

// writeBytes writes multiple bytes within an I²C transaction.
//
// Does not touch D3~D7.
func (d *i2cBus) writeBytes(w []byte) error {
	// TODO(maruel): d.pullUp
	dir := d.f.dbus.direction
	v := d.f.dbus.value
	// TODO(maruel): WAT?
	if err := d.f.h.Flush(); err != nil {
		return err
	}
	// TODO(maruel): Implement both with and without NAK check.
	var r [1]byte
	cmd := [...]byte{
		// Data out, the 0 will be replaced with the byte.
		dataOut | dataOutFall, 0, 0, 0,
		// Set back to idle.
		gpioSetD, v | i2cSCL | i2cSDAOut, dir,
		// Read ACK/NAK.
		dataIn | dataBit, 0,
		flush,
	}
	for _, c := range w {
		cmd[3] = c
		if _, err := d.f.h.Write(cmd[:]); err != nil {
			return err
		}
		if _, err := d.f.h.ReadAll(context.Background(), r[:]); err != nil {
			return err
		}
		if r[0]&1 == 0 {
			return errors.New("got NAK")
		}
	}
	return nil
}

// readBytes reads multiple bytes within an I²C transaction.
//
// Does not touch D3~D7.
func (d *i2cBus) readBytes(r []byte) error {
	// TODO(maruel): d.pullUp
	dir := d.f.dbus.direction
	v := d.f.dbus.value

	cmd := [...]byte{
		// Read 8 bits.
		dataIn | dataBit, 7,
		// Send ACK/NAK.
		dataOut | dataOutFall | dataBit, 0, 0,
		// Set back to idle.
		gpioSetD, v | i2cSCL | i2cSDAOut, dir,
		// Force read buffer flush. This is only necessary if NAK are not ignored.
		flush,
	}
	for i := range r {
		if i == len(r)-1 {
			// NAK.
			cmd[4] = 0x80
		}
		if _, err := d.f.h.Write(cmd[:]); err != nil {
			return err
		}
		if _, err := d.f.h.ReadAll(context.Background(), r[i:1]); err != nil {
			return err
		}
	}
	return nil
}

var _ i2c.BusCloser = &i2cBus{}
var _ i2c.Pins = &i2cBus{}
