// Copyright 2021 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ftdi

import (
	"context"
	"errors"
	"fmt"
	"io"

	"periph.io/x/conn/v3/physic"
	"periph.io/x/d2xx"
)

//

// bitMode is used by SetBitMode to change the chip behavior.
type bitMode uint8

const (
	// Resets all Pins to their default value
	bitModeReset bitMode = 0x00
	// Sets the DBus to asynchronous bit-bang.
	bitModeAsyncBitbang bitMode = 0x01
	// Switch to MPSSE mode (FT2232, FT2232H, FT4232H and FT232H).
	bitModeMpsse bitMode = 0x02
	// Sets the DBus to synchronous bit-bang (FT232R, FT245R, FT2232, FT2232H,
	// FT4232H and FT232H).
	bitModeSyncBitbang bitMode = 0x04
	// Switch to MCU host bus emulation (FT2232, FT2232H, FT4232H and FT232H).
	bitModeMcuHost bitMode = 0x08
	// Switch to fast opto-isolated serial mode (FT2232, FT2232H, FT4232H and
	// FT232H).
	bitModeFastSerial bitMode = 0x10
	// Sets the CBus in 4 bits bit-bang mode (FT232R and FT232H)
	// In this case, upper nibble controls which pin is output/input, lower
	// controls which of outputs are high and low.
	bitModeCbusBitbang bitMode = 0x20
	// Single Channel Synchronous 245 FIFO mode (FT2232H and FT232H).
	bitModeSyncFifo bitMode = 0x40
)

// numDevices returns the number of detected devices.
func numDevices() (int, error) {
	num, e := d2xx.CreateDeviceInfoList()
	if e != 0 {
		return 0, toErr("GetNumDevices initialization failed", e)
	}
	return num, nil
}

func openHandle(opener func(i int) (d2xx.Handle, d2xx.Err), i int) (*handle, error) {
	h, e := opener(i)
	if e != 0 {
		return nil, toErr("Open", e)
	}
	// For debugging:
	// d := &handle{h: &d2xxtest.Log{H: h, Printf: log.Printf}}
	d := &handle{h: h}
	t, vid, did, e := h.GetDeviceInfo()
	if e != 0 {
		_ = d.Close()
		return nil, toErr("GetDeviceInfo", e)
	}
	d.t = DevType(t)
	d.venID = vid
	d.devID = did
	return d, nil
}

// handle is a thin wrapper around the low level d2xx device handle to make it
// more go-idiomatic.
//
// It also implements many utility functions to help with initialization and
// device management.
type handle struct {
	// It is just above 'handle' which directly maps to D2XX function calls.
	//
	// Dev converts the int error type into Go native error and handles higher
	// level functionality like reading and writing to the USB connection.
	//
	// The content of the struct is immutable after initialization.
	h     d2xx.Handle
	t     DevType
	venID uint16
	devID uint16
}

func (h *handle) Close() error {
	// Not yet called.
	return toErr("Close", h.h.Close())
}

// Init is the general setup for common devices.
//
// It tries first the 'happy path' which doesn't reset the device. By doing so,
// the goal is to reduce the amount of glitches on the GPIO pins, on a best
// effort basis. On all devices, the GPIOs are still reset as inputs, since
// there is no way to determine if each GPIO is an input or output.
func (h *handle) Init() error {
	// Driver: maximum packet size. Note that this clears any data in the buffer,
	// so it is good to do it immediately after a reset. The 'out' parameter is
	// ignored.
	// TODO(maruel): The FT232H doc claims a 512 byte packets support in hi-speed
	// mode, which means that this would likely be better to use this value.
	if e := h.h.SetUSBParameters(65536, 0); e != 0 {
		return toErr("SetUSBParameters", e)
	}
	// Driver: Set I/O timeouts to 15 sec. The reason is that we want the
	// timeouts to be very visible, at least as the driver is being developed.
	if e := h.h.SetTimeouts(15000, 15000); e != 0 {
		return toErr("SetTimeouts", e)
	}
	// Not sure: Disable event/error characters.
	if e := h.h.SetChars(0, false, 0, false); e != 0 {
		return toErr("SetChars", e)
	}
	// Not sure: Latency timer at 1ms.
	if e := h.h.SetLatencyTimer(1); e != 0 {
		return toErr("SetLatencyTimer", e)
	}
	return nil
}

// InitNonMPSSE does initialization that should only be done when not in MPSSE
// mode.
func (h *handle) InitNonMPSSE() error {
	// Not sure: Turn on flow control to synchronize IN requests.
	if e := h.h.SetFlowControl(); e != 0 {
		return toErr("SetFlowControl", e)
	}
	// Just in case. It's a very small cost.
	return h.Flush()
}

// Reset resets the device.
func (h *handle) Reset() error {
	if e := h.h.ResetDevice(); e != 0 {
		return toErr("Reset", e)
	}
	if err := h.SetBitMode(0, bitModeReset); err != nil {
		return err
	}
	// USB/driver: Flush any pending read buffer that had been sent by the device
	// before it reset. Do not return any error there, as the device may spew a
	// read error right after being initialized.
	_ = h.Flush()
	return nil
}

// GetBitMode returns the current bit mode.
//
// This is device-dependent.
func (h *handle) GetBitMode() (byte, error) {
	l, e := h.h.GetBitMode()
	if e != 0 {
		return 0, toErr("GetBitMode", e)
	}
	return l, nil
}

// SetBitMode change the mode of operation of the device.
//
// mask sets which pins are inputs and outputs for bitModeCbusBitbang.
func (h *handle) SetBitMode(mask byte, mode bitMode) error {
	return toErr("SetBitMode", h.h.SetBitMode(mask, byte(mode)))
}

// Flush flushes any data left in the read buffer.
func (h *handle) Flush() error {
	var buf [128]byte
	for {
		p, err := h.Read(buf[:])
		if err != nil {
			return err
		}
		if p == 0 {
			return nil
		}
	}
}

// Read returns as much as available in the read buffer without blocking.
func (h *handle) Read(b []byte) (int, error) {
	// GetQueueStatus() 60µs is relatively slow compared to Read() 4µs,
	// but surprisingly if GetQueueStatus() is *not* called, Read()
	// becomes largely slower (800µs).
	//
	// TODO(maruel): This asks for more perf testing before settling on the best
	// solution.
	// TODO(maruel): Investigate FT_GetStatus().
	p, e := h.h.GetQueueStatus()
	if p == 0 || e != 0 {
		return int(p), toErr("Read/GetQueueStatus", e)
	}
	v := int(p)
	if v > len(b) {
		v = len(b)
	}
	n, e := h.h.Read(b[:v])
	return n, toErr("Read", e)
}

// ReadAll blocks to return all the data.
//
// Similar to ioutil.ReadAll() except that it will stop if the context is
// canceled.
func (h *handle) ReadAll(ctx context.Context, b []byte) (int, error) {
	// TODO(maruel): Use FT_SetEventNotification() instead of looping when
	// waiting for bytes.
	for offset := 0; offset != len(b); {
		if ctx.Err() != nil {
			return offset, io.EOF
		}
		chunk := len(b) - offset
		if chunk > 4096 {
			chunk = 4096
		}
		n, err := h.Read(b[offset : offset+chunk])
		if offset += n; err != nil {
			return offset, err
		}
	}
	return len(b), nil
}

// WriteFast writes to the USB device.
//
// In practice this takes at least 0.1ms, which limits the effective rate.
//
// There's no guarantee that the data is all written, so it is important to
// check the return value.
func (h *handle) WriteFast(b []byte) (int, error) {
	n, e := h.h.Write(b)
	return n, toErr("Write", e)
}

// Write blocks until all data is written.
func (h *handle) Write(b []byte) (int, error) {
	for offset := 0; offset != len(b); {
		chunk := len(b) - offset
		if chunk > 4096 {
			chunk = 4096
		}
		p, err := h.WriteFast(b[offset : offset+chunk])
		if err != nil {
			return offset + p, err
		}
		if p != 0 {
			offset += p
		}
	}
	return len(b), nil
}

// ReadEEPROM reads the EEPROM.
func (h *handle) ReadEEPROM(ee *EEPROM) error {
	// The raw data size must be exactly what the device contains.
	eepromSize := h.t.EEPROMSize()
	if len(ee.Raw) < eepromSize {
		ee.Raw = make([]byte, eepromSize)
	} else if len(ee.Raw) > eepromSize {
		ee.Raw = ee.Raw[:eepromSize]
	}
	ee2 := d2xx.EEPROM{Raw: ee.Raw}
	e := h.h.EEPROMRead(uint32(h.t), &ee2)
	ee.Manufacturer = ee2.Manufacturer
	ee.ManufacturerID = ee2.ManufacturerID
	ee.Desc = ee2.Desc
	ee.Serial = ee2.Serial
	if e != 0 {
		// 15 == FT_EEPROM_NOT_PROGRAMMED
		if e != 15 {
			return toErr("EEPROMRead", e)
		}
		// It's a fresh new device. Devices bought via Adafruit already have
		// their EEPROM programmed with Adafruit branding but fake devices sold by
		// CJMCU are not. Since GetDeviceInfo() above succeeded, we know the
		// device type via the USB descriptor, which is sufficient to load the
		// driver, which permits to program the EEPROM to "bootstrap" it.
		//
		// Fill it with an empty yet valid EEPROM content. We don't want to set
		// VenID or DevID to 0! Nobody would do that, right?
		ee.Raw = make([]byte, h.t.EEPROMSize())
		hdr := ee.AsHeader()
		hdr.DeviceType = h.t
		hdr.VendorID = h.venID
		hdr.ProductID = h.devID
	}
	return nil
}

// WriteEEPROM programs the EEPROM.
func (h *handle) WriteEEPROM(ee *EEPROM) error {
	if err := ee.Validate(); err != nil {
		return err
	}
	if len(ee.Raw) != 0 {
		hdr := ee.AsHeader()
		if hdr == nil {
			return errors.New("ftdi: unexpected EEPROM header size")
		}
		if hdr.DeviceType != h.t {
			return errors.New("ftdi: unexpected device type set while programming EEPROM")
		}
		if hdr.VendorID != h.venID {
			return errors.New("ftdi: unexpected VenID set while programming EEPROM")
		}
		if hdr.ProductID != h.devID {
			return errors.New("ftdi: unexpected DevID set while programming EEPROM")
		}
	}
	ee2 := d2xx.EEPROM{
		Raw:            ee.Raw,
		Manufacturer:   ee.Manufacturer,
		ManufacturerID: ee.ManufacturerID,
		Desc:           ee.Desc,
		Serial:         ee.Serial,
	}
	return toErr("EEPROMWrite", h.h.EEPROMProgram(&ee2))
}

// EraseEEPROM erases all the EEPROM.
//
// Will fail on FT232R and FT245R.
func (h *handle) EraseEEPROM() error {
	return toErr("EraseEE", h.h.EraseEE())
}

// ReadUA reads the EEPROM user area.
//
// May return nil when there's nothing programmed yet.
func (h *handle) ReadUA() ([]byte, error) {
	size, e := h.h.EEUASize()
	if e != 0 {
		return nil, toErr("EEUASize", e)
	}
	if size == 0 {
		// Happens on uninitialized EEPROM.
		return nil, nil
	}
	b := make([]byte, size)
	if e := h.h.EEUARead(b); e != 0 {
		return nil, toErr("EEUARead", e)
	}
	return b, nil
}

// WriteUA writes to the EEPROM user area.
func (h *handle) WriteUA(ua []byte) error {
	size, e := h.h.EEUASize()
	if e != 0 {
		return toErr("EEUASize", e)
	}
	if size == 0 {
		return errors.New("ftdi: please program EEPROM first")
	}
	if size < len(ua) {
		return fmt.Errorf("ftdi: maximum user area size is %d bytes", size)
	}
	if size != len(ua) {
		b := make([]byte, size)
		copy(b, ua)
		ua = b
	}
	if e := h.h.EEUAWrite(ua); e != 0 {
		return toErr("EEUAWrite", e)
	}
	return nil
}

// SetBaudRate sets the baud rate.
func (h *handle) SetBaudRate(f physic.Frequency) error {
	if f >= physic.GigaHertz {
		return errors.New("ftdi: baud rate too high")
	}
	v := uint32(f / physic.Hertz)
	return toErr("SetBaudRate", h.h.SetBaudRate(v))
}

//

func toErr(s string, e d2xx.Err) error {
	if e == 0 {
		return nil
	}
	return errors.New("ftdi: " + s + ": " + e.String())
}
