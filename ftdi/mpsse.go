// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// MPSSE is Multi-Protocol Synchronous Serial Engine
//
// MPSSE basics:
// http://www.ftdichip.com/Support/Documents/AppNotes/AN_135_MPSSE_Basics.pdf
//
// MPSSE and MCU emulation modes:
// http://www.ftdichip.com/Support/Documents/AppNotes/AN_108_Command_Processor_for_MPSSE_and_MCU_Host_Bus_Emulation_Modes.pdf

package ftdi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
)

const (
	// TDI/TDO serial operation synchronised on clock edges.
	//
	// Long streams (default):
	// - [1, 65536] bytes (length is sent minus one, requires 8 bits multiple)
	//   <op>, <LengthLow-1>, <LengthHigh-1>, <byte0>, ..., <byteN>
	//
	// Short streams (dataBit is specified):
	// - [1, 8] bits
	//   <op>, <Length-1>, <byte>
	//
	// When both dataOut and dataIn are specified, one of dataOutFall or
	// dataInFall should be specified, at least for most sane protocols.
	//
	// Flags:
	dataOut     byte = 0x10 // Enable output, default on +VE (Rise)
	dataIn      byte = 0x20 // Enable input, default on +VE (Rise)
	dataOutFall byte = 0x01 // instead of Rise
	dataInFall  byte = 0x04 // instead of Rise
	dataLSBF    byte = 0x08 // instead of MSBF
	dataBit     byte = 0x02 // instead of Byte

	// Data line drives low when the data is 0 and tristates on data 1. This is
	// used with I²C.
	// <op>, <ADBus pins>, <ACBus pins>
	dataTristate byte = 0x9E

	// TSM operation (for JTAG).
	//
	// - Send bits 6 to 0 to the TMS pin using LSB or MSB.
	// - Bit 7 is passed to TDI/DO before the first clock of TMS and is held
	//   static for the duration of TMS clocking.
	//
	// <op>, <Length>, <byte>
	tmsOutLSBFRise byte = 0x4A
	tmsOutLSBFFall byte = 0x4B
	tmsIOLSBInRise byte = 0x6A
	tmsIOLSBInFall byte = 0x6B
	// Unclear: 0x6E and 0x6F

	// GPIO operation.
	//
	// - Operates on 8 GPIOs at a time, e.g. C0~C7 or D0~D7.
	// - Direction 1 means output, 0 means input.
	//
	// <op>, <value>, <direction>
	gpioSetD byte = 0x80
	gpioSetC byte = 0x82
	// <op>, returns <value>
	gpioReadD byte = 0x81
	gpioReadC byte = 0x83

	// Internal loopback.
	//
	// Connects TDI and TDO together.
	internalLoopbackEnable  byte = 0x84
	internalLoopbackDisable byte = 0x85

	// Clock.
	//
	// The TCK/SK has a 50% duty cycle.
	//
	// The inactive clock state can be set via the gpioSetD command and control
	// bit 0.
	//
	// By default, the base clock is 6MHz via a 5x divisor. On
	// FT232H/FT2232H/FT4232H, the 5x divisor can be disabled.
	clock30MHz byte = 0x8A
	clock6MHz  byte = 0x8B
	// Sets clock divisor.
	//
	// The effective value depends if clock30MHz was sent or not.
	//
	// - 0(1) 6MHz / 30MHz
	// - 1(2) 3MHz / 15MHz
	// - 2(3) 2MHz / 10MHz
	// - 3(4) 1.5MHz / 7.5MHz
	// - 4(5) 1.25MHz / 6MHz
	// - ...
	// - 0xFFFF(65536) 91.553Hz / 457.763Hz
	//
	// <op>, <valueL-1>, <valueH-1>
	clockSetDivisor byte = 0x86
	// Uses 3 phases data clocking: data is valid on both clock edges. Needed
	// for I²C.
	clock3Phase byte = 0x8C
	// Uses normal 2 phases data clocking.
	clock2Phase byte = 0x8D
	// Enables clock even while not doing any operation. Used with JTAG.
	// Enables the clock between [1, 8] pulses.
	// <op>, <length-1>
	clockOnShort byte = 0x8E
	// Enables the clock between [8, 524288] pulses in 8 multiples.
	// <op>, <lengthL-1>, <lengthH-1>
	clockOnLong byte = 0x8F
	// Enables clock until D5 is high or low. Used with JTAG.
	clockUntilHigh byte = 0x94
	clockUntilLow  byte = 0x95
	// <op>, <lengthL-1>, <lengthH-1> in 8 multiples.
	clockUntilHighLong byte = 0x9C
	clockUntilLowLong  byte = 0x9D
	// Enables adaptive clocking. Used with JTAG.
	//
	// This causes the controller to wait for D7 signal state as an ACK.
	clockAdaptive byte = 0x96
	// Disables adaptive clocking.
	clockNormal byte = 0x97

	// CPU mode.
	//
	// Access the device registers like a memory mapped device.
	//
	// <op>, <addrLow>
	cpuReadShort byte = 0x90
	// <op>, <addrHi>, <addrLow>
	cpuReadFar byte = 0x91
	// <op>, <addrLow>, <data>
	cpuWriteShort byte = 0x92
	// <op>, <addrHi>, <addrLow>, <data>
	cpuWriteFar byte = 0x91

	// Buffer operations.
	//
	// Flush the buffer back to the host.
	flush byte = 0x87
	// Wait until D5 (JTAG) or I/O1 (CPU) is high. Once it is detected as
	// high, the MPSSE engine moves on to process the next instruction.
	waitHigh byte = 0x88
	waitLow  byte = 0x89
)

// InitMPSSE sets the device into MPSSE mode.
//
// This requires a f232h, ft2232, ft2232h or a ft4232h.
//
// Use only one of Init or InitMPSSE.
func (h *handle) InitMPSSE() error {
	// http://www.ftdichip.com/Support/Documents/AppNotes/AN_255_USB%20to%20I2C%20Example%20using%20the%20FT232H%20and%20FT201X%20devices.pdf
	// Pre-state:
	// - Write EEPROM i.IsFifo = true so the device DBus is started in tristate.

	// Try to verify the MPSSE controller without initializing it first. This is
	// the 'happy path', which enables reusing the device is its current state
	// without affecting current GPIO state.
	if h.mpsseVerify() != nil {
		// Do a full reset. Just trying to set the MPSSE controller will
		// likely not work. That's a layering violation (since the retry with reset
		// is done in driver.go) but we've survived worse things...
		//
		// TODO(maruel): This is not helping in practice, this need to be fine
		// tuned.
		if err := h.Reset(); err != nil {
			return err
		}
		if err := h.Init(); err != nil {
			return err
		}
		// That does the magic thing.
		if err := h.SetBitMode(0, bitModeMpsse); err != nil {
			return err
		}
		if err := h.mpsseVerify(); err != nil {
			return err
		}
	}

	// Initialize MPSSE to a known state.
	// Reset the clock since it is impossible to read back the current clock rate.
	// Reset all the GPIOs are inputs since it is impossible to read back the
	// state of each GPIO (if they are input or output).
	cmd := []byte{
		clock30MHz, clockNormal, clock2Phase, internalLoopbackDisable,
		gpioSetC, 0x00, 0x00,
		gpioSetD, 0x00, 0x00,
	}
	if _, err := h.Write(cmd); err != nil {
		return err
	}
	// Success!!
	return nil
}

// mpsseVerify sends an invalid MPSSE command and verifies the returned value
// is incorrect.
//
// In practice this takes around 2ms.
func (h *handle) mpsseVerify() error {
	var b [2]byte
	for _, v := range []byte{0xAA, 0xAB} {
		// Write a bad command and ensure it returned correctly.
		// Unlike what the application note proposes, include a flush op right
		// after. Without the flush, the device will only flush after the delay
		// specified to SetLatencyTimer. The flush removes this unneeded wait,
		// which enables increasing the delay specified to SetLatencyTimer.
		b[0] = v
		b[1] = flush
		if _, err := h.Write(b[:]); err != nil {
			return fmt.Errorf("ftdi: MPSSE verification failed: %w", err)
		}
		p, e := h.h.GetQueueStatus()
		if e != 0 {
			return toErr("Read/GetQueueStatus", e)
		}
		if p != 2 {
			return fmt.Errorf("ftdi: MPSSE verification failed: expected 2 bytes reply, got %d bytes", p)
		}
		ctx, cancel := context200ms()
		defer cancel()
		if _, err := h.ReadAll(ctx, b[:]); err != nil {
			return fmt.Errorf("ftdi: MPSSE verification failed: %w", err)
		}
		// 0xFA means invalid command, 0xAA is the command echoed back.
		if b[0] != 0xFA || b[1] != v {
			return fmt.Errorf("ftdi: MPSSE verification failed test for byte %#x: %#x", v, b)
		}
	}
	return nil
}

//

// MPSSERegRead reads the memory mapped registers from the device.
func (h *handle) MPSSERegRead(addr uint16) (byte, error) {
	// Unlike most other operations, the uint16 byte order is <hi>, <lo>.
	b := [...]byte{cpuReadFar, byte(addr >> 8), byte(addr), flush}
	if _, err := h.Write(b[:]); err != nil {
		return 0, err
	}
	ctx, cancel := context200ms()
	defer cancel()
	_, err := h.ReadAll(ctx, b[:1])
	return b[0], err
}

// MPSSEClock sets the clock at the closest value and returns it.
func (h *handle) MPSSEClock(f physic.Frequency) (physic.Frequency, error) {
	// TODO(maruel): Memory clock and skip if the same value.
	clk := clock30MHz
	base := 30 * physic.MegaHertz
	div := base / f
	if div >= 65536 {
		clk = clock6MHz
		base /= 5
		div = base / f
		if div >= 65536 {
			return 0, errors.New("ftdi: clock frequency is too low")
		}
	}
	b := [...]byte{clk, clockSetDivisor, byte(div - 1), byte((div - 1) >> 8)}
	_, err := h.Write(b[:])
	return base / div, err
}

// mpsseTxOp returns the right MPSSE command byte for the stream.
func mpsseTxOp(w, r bool, ew, er gpio.Edge, lsbf bool) byte {
	op := byte(0)
	if lsbf {
		op |= dataLSBF
	}
	if w {
		op |= dataOut
		if ew == gpio.FallingEdge {
			op |= dataOutFall
		}
	}
	if r {
		op |= dataIn
		if er == gpio.FallingEdge {
			op |= dataInFall
		}
	}
	return op
}

// MPSSETx runs a transaction on the clock on pins D0, D1 and D2.
//
// It can only do it on a multiple of 8 bits.
func (h *handle) MPSSETx(w, r []byte, ew, er gpio.Edge, lsbf bool) error {
	l := len(w)
	if len(w) != 0 {
		// TODO(maruel): This is easy to fix by daisy chaining operations.
		if len(w) > 65536 {
			return errors.New("ftdi: write buffer too long; max 65536")
		}
	}
	if len(r) != 0 {
		if len(r) > 65536 {
			return errors.New("ftdi: read buffer too long; max 65536")
		}
		if l != 0 && len(r) != l {
			return errors.New("ftdi: mismatched buffer lengths")
		}
		l = len(r)
	}
	// The FT232H has 1Kb Tx and Rx buffers. So partial writes should be done.
	// TODO(maruel): Test.

	// Flush can be useful if rbits != 0.
	op := mpsseTxOp(len(w) != 0, len(r) != 0, ew, er, lsbf)
	cmd := []byte{op, byte(l - 1), byte((l - 1) >> 8)}
	cmd = append(cmd, w...)
	cmd = append(cmd, flush)
	if _, err := h.Write(cmd); err != nil {
		return err
	}
	if len(r) != 0 {
		ctx, cancel := context200ms()
		defer cancel()
		_, err := h.ReadAll(ctx, r)
		return err
	}
	return nil
}

// MPSSETxShort runs a transaction on the clock pins D0, D1 and D2 for a byte
// or less: between 1 and 8 bits.
func (h *handle) MPSSETxShort(w byte, wbits, rbits int, ew, er gpio.Edge, lsbf bool) (byte, error) {
	op := byte(dataBit)
	if lsbf {
		op |= dataLSBF
	}
	l := wbits
	if wbits != 0 {
		if wbits > 8 {
			return 0, errors.New("ftdi: write buffer too long; max 8")
		}
		op |= dataOut
		if ew == gpio.FallingEdge {
			op |= dataOutFall
		}
	}
	if rbits != 0 {
		if rbits > 8 {
			return 0, errors.New("ftdi: read buffer too long; max 8")
		}
		op |= dataIn
		if er == gpio.FallingEdge {
			op |= dataInFall
		}
		if l != 0 && rbits != l {
			return 0, errors.New("ftdi: mismatched buffer lengths")
		}
		l = rbits
	}
	b := [3]byte{op, byte(l - 1)}
	cmd := b[:2]
	if wbits != 0 {
		cmd = append(cmd, w)
	}
	if rbits != 0 {
		cmd = append(cmd, flush)
	}
	if _, err := h.Write(cmd); err != nil {
		return 0, err
	}
	if rbits != 0 {
		ctx, cancel := context200ms()
		defer cancel()
		_, err := h.ReadAll(ctx, b[:1])
		return b[0], err
	}
	return 0, nil
}

// MPSSECBus operates on 8 GPIOs at a time C0~C7.
//
// Direction 1 means output, 0 means input.
func (h *handle) MPSSECBus(mask, value byte) error {
	b := [...]byte{gpioSetC, value, mask}
	_, err := h.Write(b[:])
	return err
}

// MPSSEDBus operates on 8 GPIOs at a time D0~D7.
//
// Direction 1 means output, 0 means input.
func (h *handle) MPSSEDBus(mask, value byte) error {
	b := [...]byte{gpioSetD, value, mask}
	_, err := h.Write(b[:])
	return err
}

// MPSSECBusRead reads all the CBus pins C0~C7.
func (h *handle) MPSSECBusRead() (byte, error) {
	b := [...]byte{gpioReadC, flush}
	if _, err := h.Write(b[:]); err != nil {
		return 0, err
	}
	ctx, cancel := context200ms()
	defer cancel()
	if _, err := h.ReadAll(ctx, b[:1]); err != nil {
		return 0, err
	}
	return b[0], nil
}

// MPSSEDBusRead reads all the DBus pins D0~D7.
func (h *handle) MPSSEDBusRead() (byte, error) {
	b := [...]byte{gpioReadD, flush}
	if _, err := h.Write(b[:]); err != nil {
		return 0, err
	}
	ctx, cancel := context200ms()
	defer cancel()
	if _, err := h.ReadAll(ctx, b[:1]); err != nil {
		return 0, err
	}
	return b[0], nil
}

func context200ms() (context.Context, func()) {
	return context.WithTimeout(context.Background(), 200*time.Millisecond)
}
