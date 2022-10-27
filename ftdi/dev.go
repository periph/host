// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ftdi

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpiostream"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
)

// PinStreamOut is a gpio pin that supports raw data stream output.
type PinStreamOut interface {
	gpio.PinIO
	// StreamOut defines gpiostream.PinOut.
	StreamOut(s gpiostream.Stream) error
}

// Info is the information gathered about the connected FTDI device.
//
// The data is gathered from the USB descriptor.
type Info struct {
	// Opened is true if the device was successfully opened.
	Opened bool
	// Type is the FTDI device type.
	//
	// The value can be "FT232H", "FT232R", etc.
	//
	// An empty string means the type is unknown.
	Type string
	// VenID is the vendor ID from the USB descriptor information. It is expected
	// to be 0x0403 (FTDI).
	VenID uint16
	// DevID is the product ID from the USB descriptor information. It is
	// expected to be one of 0x6001, 0x6006, 0x6010, 0x6014.
	DevID uint16
}

// Dev represents one FTDI device.
//
// There can be multiple FTDI devices connected to a host.
//
// The device may also export one or multiple of I²C, SPI buses. You need to
// either cast into the right hardware, but more simply use the i2creg / spireg
// bus/port registries.
type Dev interface {
	// conn.Resource
	String() string
	Halt() error

	// Info returns information about an opened device.
	Info(i *Info)

	// Header returns the GPIO pins exposed on the chip.
	Header() []gpio.PinIO

	// SetSpeed sets the base clock for all I/O transactions.
	//
	// The device defaults to its fastest speed.
	SetSpeed(f physic.Frequency) error

	// EEPROM returns the EEPROM content.
	EEPROM(ee *EEPROM) error
	// WriteEEPROM updates the EEPROM. Must be used carefully.
	WriteEEPROM(ee *EEPROM) error
	// EraseEEPROM erases the EEPROM. Must be used carefully.
	EraseEEPROM() error
	// UserArea reads and return the EEPROM part that can be used to stored user
	// defined values.
	UserArea() ([]byte, error)
	// WriteUserArea updates the user area in the EEPROM.
	//
	// If the length of ua is less than the available space, is it zero extended.
	WriteUserArea(ua []byte) error
}

// broken represents a device that couldn't be opened correctly.
//
// It returns an error message to help the user diagnose issues.
type broken struct {
	index int
	err   error
	name  string
}

func (b *broken) String() string {
	return b.name
}

func (b *broken) Halt() error {
	return nil
}

func (b *broken) Info(i *Info) {
	i.Opened = false
}

func (b *broken) Header() []gpio.PinIO {
	return nil
}

func (b *broken) SetSpeed(f physic.Frequency) error {
	return b.err
}

func (b *broken) EEPROM(ee *EEPROM) error {
	return b.err
}

func (b *broken) WriteEEPROM(ee *EEPROM) error {
	return b.err
}

func (b *broken) EraseEEPROM() error {
	return b.err
}

func (b *broken) UserArea() ([]byte, error) {
	return nil, b.err
}

func (b *broken) WriteUserArea(ua []byte) error {
	return b.err
}

// generic represents a generic FTDI device.
//
// It is used for the models that this package doesn't fully support yet.
type generic struct {
	// Immutable after initialization.
	index int
	h     *handle
	name  string
}

func (f *generic) String() string {
	return f.name
}

// Halt implements conn.Resource.
//
// This halts all operations going through this device.
func (f *generic) Halt() error {
	return f.h.Reset()
}

// Info returns information about an opened device.
func (f *generic) Info(i *Info) {
	i.Opened = true
	i.Type = f.h.t.String()
	i.VenID = f.h.venID
	i.DevID = f.h.devID
}

// Header returns the GPIO pins exposed on the chip.
func (f *generic) Header() []gpio.PinIO {
	return nil
}

func (f *generic) SetSpeed(freq physic.Frequency) error {
	// TODO(maruel): Doc says the actual speed is 16x, confirm.
	return f.h.SetBaudRate(freq)
}

func (f *generic) EEPROM(ee *EEPROM) error {
	return f.h.ReadEEPROM(ee)
	/*
		if f.ee.Raw == nil {
			if err := f.h.readEEPROM(&f.ee); err != nil {
				return nil
			}
			if f.ee.Raw == nil {
				// It's a fresh new device. Devices bought via Adafruit already have
				// their EEPROM programmed with Adafruit branding but devices sold by
				// CJMCU are not. Since d2xxGetDeviceInfo() above succeeded, we know the
				// device type via the USB descriptor, which is sufficient to load the
				// driver, which permits to program the EEPROM to "bootstrap" it.
				f.ee.Raw = []byte{}
			}
		}
		*ee = f.ee
		return nil
	*/
}

func (f *generic) WriteEEPROM(ee *EEPROM) error {
	// TODO(maruel): Compare with the cached EEPROM, and only update the
	// different values if needed so reduce the EEPROM wear.
	// f.h.h.d2xxWriteEE()
	return f.h.WriteEEPROM(ee)
}

func (f *generic) EraseEEPROM() error {
	return f.h.EraseEEPROM()
}

func (f *generic) UserArea() ([]byte, error) {
	return f.h.ReadUA()
}

func (f *generic) WriteUserArea(ua []byte) error {
	return f.h.WriteUA(ua)
}

//

func newFT232H(g generic) (*FT232H, error) {
	f := &FT232H{
		generic: g,
		cbus:    gpiosMPSSE{h: g.h, cbus: true},
		dbus:    gpiosMPSSE{h: g.h},
		c8:      invalidPin{num: 16, n: g.name + ".C8"}, // , dp: gpio.PullUp
		c9:      invalidPin{num: 17, n: g.name + ".C9"}, // , dp: gpio.PullUp
	}
	f.cbus.init(f.name)
	f.dbus.init(f.name)

	for i := range f.dbus.pins {
		f.hdr[i] = &f.dbus.pins[i]
	}
	for i := range f.cbus.pins {
		f.hdr[i+8] = &f.cbus.pins[i]
	}
	// TODO(maruel): C8 and C9 can be used when their mux in the EEPROM is set to
	// ft232hCBusIOMode.
	f.hdr[16] = &f.c8
	f.hdr[17] = &f.c9
	f.D0 = f.hdr[0]
	f.D1 = &f.dbus.pins[1]
	f.D2 = f.hdr[2]
	f.D3 = f.hdr[3]
	f.D4 = f.hdr[4]
	f.D5 = f.hdr[5]
	f.D6 = f.hdr[6]
	f.D7 = f.hdr[7]
	f.C0 = f.hdr[8]
	f.C1 = f.hdr[9]
	f.C2 = f.hdr[10]
	f.C3 = f.hdr[11]
	f.C4 = f.hdr[12]
	f.C5 = f.hdr[13]
	f.C6 = f.hdr[14]
	f.C7 = f.hdr[15]
	f.C8 = f.hdr[16]
	f.C9 = f.hdr[17]

	// This function forces all pins as inputs.
	if err := f.h.InitMPSSE(); err != nil {
		return nil, err
	}
	f.s.c.f = f
	f.i.f = f
	return f, nil
}

// FT232H represents a FT232H device.
//
// It implements Dev.
//
// The FT232H has 1024 bytes output buffer and 1024 bytes input buffer. It
// supports 512 bytes USB packets.
//
// The device can be used in a few different modes, two modes are supported:
//
// - D0~D3 as a serial protocol (MPSEE), supporting I²C and SPI (and eventually
// UART), In this mode, D4~D7 and C0~C7 can be used as synchronized GPIO.
//
// - D0~D7 as a synchronous 8 bits bit-bang port. In this mode, only a few pins
// on CBus are usable in slow mode.
//
// Each group of pins D0~D7 and C0~C7 can be changed at once in one pass via
// DBus() or CBus().
//
// This enables usage as an 8 bit parallel port.
//
// Pins C8 and C9 can only be used in 'slow' mode via EEPROM and are currently
// not implemented.
//
// # Datasheet
//
// http://www.ftdichip.com/Support/Documents/DataSheets/ICs/DS_FT232H.pdf
type FT232H struct {
	generic

	D0 gpio.PinIO   // Clock output
	D1 PinStreamOut // Data out
	D2 gpio.PinIO   // Data in
	D3 gpio.PinIO   // Chip select
	D4 gpio.PinIO
	D5 gpio.PinIO
	D6 gpio.PinIO
	D7 gpio.PinIO
	C0 gpio.PinIO
	C1 gpio.PinIO
	C2 gpio.PinIO
	C3 gpio.PinIO
	C4 gpio.PinIO
	C5 gpio.PinIO
	C6 gpio.PinIO
	C7 gpio.PinIO
	C8 gpio.PinIO // Not implemented
	C9 gpio.PinIO // Not implemented

	hdr  [18]gpio.PinIO
	cbus gpiosMPSSE
	dbus gpiosMPSSE
	c8   invalidPin // gpio.PullUp
	c9   invalidPin // gpio.PullUp

	mu       sync.Mutex
	usingI2C bool
	usingSPI bool
	i        i2cBus
	s        spiMPSEEPort
	// TODO(maruel): Technically speaking, a SPI port could be hacked up too in
	// sync bit-bang but there's less point when MPSEE is available.
}

// Header returns the GPIO pins exposed on the chip.
func (f *FT232H) Header() []gpio.PinIO {
	out := make([]gpio.PinIO, len(f.hdr))
	copy(out, f.hdr[:])
	return out
}

func (f *FT232H) SetSpeed(freq physic.Frequency) error {
	// TODO(maruel): When using MPSEE, use the MPSEE command. If using sync
	// bit-bang, use SetBaudRate().

	// TODO(maruel): Doc says the actual speed is 16x, confirm.
	return f.h.SetBaudRate(freq)
}

// CBus sets the values of C0 to C7 in the specified direction and value.
//
// 0 direction means input, 1 means output.
func (f *FT232H) CBus(direction, value byte) error {
	return f.h.MPSSECBus(direction, value)
}

// DBus sets the values of D0 to d7 in the specified direction and value.
//
// 0 direction means input, 1 means output.
//
// This function must be used to set Clock idle level.
func (f *FT232H) DBus(direction, value byte) error {
	return f.h.MPSSEDBus(direction, value)
}

// CBusRead reads the values of C0 to C7.
func (f *FT232H) CBusRead() (byte, error) {
	return f.h.MPSSECBusRead()
}

// DBusRead reads the values of D0 to D7.
func (f *FT232H) DBusRead() (byte, error) {
	return f.h.MPSSEDBusRead()
}

// I2C returns an I²C bus over the AD bus.
//
// pull can be either gpio.PullUp or gpio.Float. The recommended pull up
// resistors are 10kΩ for 100kHz and 2kΩ for 400kHz when using Float. The
// GPIO's pull up is 75kΩ, which may require using a lower speed for signal
// reliability. Optimal pull up resistor calculation depends on the capacitance.
//
// It uses D0, D1 and D2.
//
// D0 is SCL. It must to be pulled up externally.
//
// D1 and D2 are used for SDA. D1 is the output using open drain, D2 is the
// input. D1 and D2 must be wired together and must be pulled up externally.
//
// It is recommended to set the mode to ‘245 FIFO’ in the EEPROM of the FT232H.
//
// The FIFO mode is recommended because it allows the ADbus lines to start as
// tristate. If the chip starts in the default UART mode, then the ADbus lines
// will be in the default UART idle states until the application opens the port
// and configures it as MPSSE. Care should also be taken that the RD# input on
// ACBUS is not asserted in this initial state as this can cause the FIFO lines
// to drive out.
func (f *FT232H) I2C(pull gpio.Pull) (i2c.BusCloser, error) {
	if pull != gpio.PullUp && pull != gpio.Float {
		return nil, errors.New("d2xx: I²C pull can only be PullUp or Float")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usingI2C {
		return nil, errors.New("d2xx: already using I²C")
	}
	if f.usingSPI {
		return nil, errors.New("d2xx: already using SPI")
	}
	if err := f.i.setupI2C(pull == gpio.PullUp); err != nil {
		_ = f.i.stopI2C()
		return nil, err
	}
	return &f.i, nil
}

// SPI returns a SPI port over the AD bus.
//
// It uses D0, D1, D2 and D3. D0 is the clock, D1 the output (MOSI), D2 is the
// input (MISO) and D3 is CS line.
func (f *FT232H) SPI() (spi.PortCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usingI2C {
		return nil, errors.New("d2xx: already using I²C")
	}
	if f.usingSPI {
		return nil, errors.New("d2xx: already using SPI")
	}
	// Don't mark it as being used yet. It only become used once Connect() is
	// called.
	return &f.s, nil
}

//

func newFT232R(g generic) (*FT232R, error) {
	f := &FT232R{
		generic: g,
		dbus:    [...]dbusPinSync{{num: 0}, {num: 1}, {num: 2}, {num: 3}, {num: 4}, {num: 5}, {num: 6}, {num: 7}},
		cbus:    [...]cbusPin{{num: 8, p: gpio.PullUp}, {num: 9, p: gpio.PullUp}, {num: 10, p: gpio.PullUp}, {num: 11, p: gpio.Float}},
	}
	// Use the UART names, as this is how all FT232R boards are marked.
	dnames := [...]string{"TX", "RX", "RTS", "CTS", "DTR", "DSR", "DCD", "RI"}
	for i := range f.dbus {
		f.dbus[i].n = f.name + "." + dnames[i]
		f.dbus[i].bus = f
		f.hdr[i] = &f.dbus[i]
	}
	for i := range f.cbus {
		f.cbus[i].n = f.name + ".C" + strconv.Itoa(i)
		f.cbus[i].bus = f
		f.hdr[i+8] = &f.cbus[i]
	}
	f.D0 = f.hdr[0]
	f.D1 = f.hdr[1]
	f.D2 = f.hdr[2]
	f.D3 = f.hdr[3]
	f.D4 = f.hdr[4]
	f.D5 = f.hdr[5]
	f.D6 = f.hdr[6]
	f.D7 = f.hdr[7]
	f.TX = f.hdr[0]
	f.RX = f.hdr[1]
	f.RTS = f.hdr[2]
	f.CTS = f.hdr[3]
	f.DTR = f.hdr[4]
	f.DSR = f.hdr[5]
	f.DCD = f.hdr[6]
	f.RI = f.hdr[7]
	f.C0 = f.hdr[8]
	f.C1 = f.hdr[9]
	f.C2 = f.hdr[10]
	f.C3 = f.hdr[11]

	if err := f.h.InitNonMPSSE(); err != nil {
		return nil, err
	}

	// Default to 3MHz.
	if err := f.h.SetBaudRate(3 * physic.MegaHertz); err != nil {
		return nil, err
	}

	// Set all CBus pins as input.
	if err := f.h.SetBitMode(0, bitModeCbusBitbang); err != nil {
		return nil, err
	}
	// And read their value.
	// TODO(maruel): Sadly this is impossible to know which pin is input or
	// output, but we could try to guess, as the call above may generate noise on
	// the line which could interfere with the device connected.
	var err error
	if f.cbusnibble, err = f.h.GetBitMode(); err != nil {
		return nil, err
	}
	// Set all DBus as asynchronous bitbang, everything as input.
	if err := f.h.SetBitMode(0, bitModeAsyncBitbang); err != nil {
		return nil, err
	}
	// And read their value.
	var b [1]byte
	if _, err := f.h.ReadAll(context.Background(), b[:]); err != nil {
		return nil, err
	}
	f.dvalue = b[0]
	f.s.c.f = f
	return f, nil
}

// FT232R represents a FT232RL/FT232RQ device.
//
// It implements Dev.
//
// Not all pins may be physically connected on the header!
//
// Adafruit's version only has the following pins connected: RX, TX, RTS and
// CTS.
//
// SparkFun's version exports all pins *except* (inexplicably) the CBus ones.
//
// The FT232R has 128 bytes output buffer and 256 bytes input buffer.
//
// Pin C4 can only be used in 'slow' mode via EEPROM and is currently not
// implemented.
//
// # Datasheet
//
// http://www.ftdichip.com/Support/Documents/DataSheets/ICs/DS_FT232R.pdf
type FT232R struct {
	generic

	// Pin and their alias to the Dn pins for user convenience. Each pair points
	// to the exact same pin.
	D0, TX  gpio.PinIO // Transmit; SPI_MOSI
	D1, RX  gpio.PinIO // Receive; SPI_MISO
	D2, RTS gpio.PinIO // Request To Send Control Output / Handshake signal; SPI_CLK
	D3, CTS gpio.PinIO // Clear to Send Control input / Handshake signal; SPI_CS
	D4, DTR gpio.PinIO // Data Terminal Ready Control Output / Handshake signal
	D5, DSR gpio.PinIO // Data Set Ready Control Input / Handshake signal
	D6, DCD gpio.PinIO // Data Carrier Detect Control input
	D7, RI  gpio.PinIO // Ring Indicator Control Input. When remote wake up is enabled in the internal EEPROM taking RI# low can be used to resume the PC USB host controller from suspend.

	// The CBus pins are slower to use, but can drive an high load, like a LED.
	C0 gpio.PinIO
	C1 gpio.PinIO
	C2 gpio.PinIO
	C3 gpio.PinIO

	dbus [8]dbusPinSync
	cbus [4]cbusPin
	hdr  [12]gpio.PinIO

	// Mutable.
	mu         sync.Mutex
	usingSPI   bool
	usingCBus  bool
	s          spiSyncPort
	dmask      uint8 // 0 input, 1 output
	dvalue     uint8
	cbusnibble uint8 // upper nibble is I/O control, lower nibble is values.
}

// Header returns the GPIO pins exposed on the chip.
func (f *FT232R) Header() []gpio.PinIO {
	out := make([]gpio.PinIO, len(f.hdr))
	copy(out, f.hdr[:])
	return out
}

// SetDBusMask sets all D0~D7 input or output mode at once.
//
// mask is the input/output pins to use. A bit value of 0 sets the
// corresponding pin to an input, a bit value of 1 sets the corresponding pin
// to an output.
//
// It should be called before calling Tx().
func (f *FT232R) SetDBusMask(mask uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usingSPI {
		return errors.New("d2xx: already using SPI")
	}
	return f.setDBusMaskLocked(mask)
}

// Tx does synchronized read-then-write on all the D0~D7 GPIOs.
//
// SetSpeed() determines the pace at which the I/O is done.
//
// SetDBusMask() determines which bits are interpreted in the w and r byte
// slice. w has its significant value masked by 'mask' and r has its
// significant value masked by '^mask'.
//
// Input sample is done *before* updating outputs. So r[0] is sampled before
// w[0] is used. The last w byte should be duplicated if an addition read is
// desired.
//
// On the Adafruit cable, only the first 4 bits D0(TX), D1(RX), D2(RTS) and
// D3(CTS) are connected. This is just enough to create a full duplex SPI bus!
func (f *FT232R) Tx(w, r []byte) error {
	if len(w) != 0 {
		if len(r) != 0 && len(w) != len(r) {
			return errors.New("d2xx: length of buffer w and r must match")
		}
	} else if len(r) == 0 {
		return errors.New("d2xx: at least one of w or r must be passed")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usingSPI {
		return errors.New("d2xx: already using SPI")
	}
	return f.txLocked(w, r)
}

// SPI returns a SPI port over the first 4 pins.
//
// It uses D0(TX), D1(RX), D2(RTS) and D3(CTS). D2(RTS) is the clock, D0(TX)
// the output (MOSI), D1(RX) is the input (MISO) and D3(CTS) is CS line.
func (f *FT232R) SPI() (spi.PortCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usingSPI {
		return nil, errors.New("d2xx: already using SPI")
	}
	// Don't mark it as being used yet. It only become used once Connect() is
	// called.
	return &f.s, nil
}

// setDBusMaskLocked is the locked version of SetDBusMask.
func (f *FT232R) setDBusMaskLocked(mask uint8) error {
	if mask != f.dmask {
		if err := f.h.SetBitMode(mask, bitModeAsyncBitbang); err != nil {
			return err
		}
		f.dmask = mask
	}
	return nil
}

func (f *FT232R) txLocked(w, r []byte) error {
	// Investigate FT232R clock issue:
	// http://developer.intra2net.com/mailarchive/html/libftdi/2010/msg00240.html

	// The FT232R has 128 bytes TX buffer and 256 bytes RX buffer. Chunk into 64
	// bytes chunks. That's half the buffer size of the TX buffer and permits
	// pipelining and removes the risk of buffer overrun. This is important
	// otherwise there's huge gaps due to the USB transmit overhead.
	// TODO(maruel): Determine what's optimal via experimentation.
	chunk := 64
	var scratch [128]byte
	if len(w) == 0 {
		// Read only.
		for i := range scratch {
			scratch[i] = f.dvalue
		}
		for len(r) != 0 {
			// TODO(maruel): Optimize.
			c := len(r)
			if c > chunk {
				c = chunk
			}
			if _, err := f.h.Write(scratch[:c]); err != nil {
				return err
			}
			if _, err := f.h.ReadAll(context.Background(), r[:c]); err != nil {
				return err
			}
			r = r[c:]
		}
	} else if len(r) == 0 {
		// Write only.
		// The first write is 128 bytes to fill the buffer.
		chunk = 128
		for len(w) != 0 {
			c := len(w)
			if c > chunk {
				c = chunk
			}
			if _, err := f.h.Write(w[:c]); err != nil {
				return err
			}
			w = w[c:]
			chunk = 64
		}
		/*
			// Let the USB drive pace it.
			if _, err := f.h.Write(w); err != nil {
				return err
			}
		*/
	} else {
		// R/W.
		// Always write one 'w' ahead.
		// The first write is 128 bytes to fill the buffer.
		chunk = 128
		cw := len(w)
		if cw > chunk {
			cw = chunk
		}
		if _, err := f.h.Write(w[:cw]); err != nil {
			return err
		}
		w = w[cw:]
		chunk = 64
		for len(r) != 0 {
			// Read then write.
			cr := len(r)
			if cr > chunk {
				cr = chunk
			}
			if _, err := f.h.ReadAll(context.Background(), r[:cr]); err != nil {
				return err
			}
			r = r[cr:]

			cw = len(w)
			if cw > chunk {
				cw = chunk
			}
			if cw != 0 {
				if _, err := f.h.Write(w[:cw]); err != nil {
					return err
				}
				w = w[cw:]
			}
		}
	}
	return nil
}

// dbusSyncGPIOFunc implements dbusSync. It returns the function of a GPIO
// pin.
func (f *FT232R) dbusSyncGPIOFunc(n int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usingSPI {
		switch n {
		case 0:
			return "SPI_MOSI" // TX
		case 1:
			return "SPI_MISO" // RX
		case 2:
			return "SPI_CLK" // RTS
		case 3:
			return "SPI_CS" // CTS
		}
	}
	mask := uint8(1 << uint(n))
	if f.dmask&mask != 0 {
		return "Out/" + gpio.Level(f.dvalue&mask != 0).String()
	}
	return "In/" + f.dbusSyncReadLocked(n).String()
}

// dbusSyncGPIOIn implements dbusSync.
func (f *FT232R) dbusSyncGPIOIn(n int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// TODO(maruel): if f.usingSPI && n < 4.
	mask := uint8(1 << uint(n))
	if f.dmask&mask == 0 {
		// Already input.
		return nil
	}
	v := f.dmask &^ mask
	if err := f.h.SetBitMode(v, bitModeAsyncBitbang); err != nil {
		return err
	}
	f.dmask = v
	return nil
}

// dbusSyncGPIORead implements dbusSync.
func (f *FT232R) dbusSyncGPIORead(n int) gpio.Level {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dbusSyncReadLocked(n)
}

func (f *FT232R) dbusSyncReadLocked(n int) gpio.Level {
	// In synchronous mode, to read we must write first to for a sample.
	b := [1]byte{f.dvalue}
	if _, err := f.h.Write(b[:]); err != nil {
		return gpio.Low
	}
	mask := uint8(1 << uint(n))
	if _, err := f.h.ReadAll(context.Background(), b[:]); err != nil {
		return gpio.Low
	}
	f.dvalue = b[0]
	return f.dvalue&mask != 0
}

// dbusSyncGPIOOut implements dbusSync.
func (f *FT232R) dbusSyncGPIOOut(n int, l gpio.Level) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	mask := uint8(1 << uint(n))
	if f.dmask&mask != 1 {
		// Was input.
		v := f.dmask | mask
		if err := f.h.SetBitMode(v, bitModeAsyncBitbang); err != nil {
			return err
		}
		f.dmask = v
	}
	return f.dbusSyncGPIOOutLocked(n, l)
}

func (f *FT232R) dbusSyncGPIOOutLocked(n int, l gpio.Level) error {
	b := [1]byte{f.dvalue}
	if _, err := f.h.Write(b[:]); err != nil {
		return err
	}
	f.dvalue = b[0]
	// In synchronous mode, we must read after writing to flush the buffer.
	if _, err := f.h.Write(b[:]); err != nil {
		return err
	}
	return nil
}

// cBusGPIOFunc implements cBusGPIO.
func (f *FT232R) cBusGPIOFunc(n int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmask := uint8(0x10 << uint(n))
	vmask := uint8(1 << uint(n))
	if f.cbusnibble&fmask != 0 {
		return "Out/" + gpio.Level(f.cbusnibble&vmask != 0).String()
	}
	return "In/" + f.cBusReadLocked(n).String()
}

// cBusGPIOIn implements cBusGPIO.
func (f *FT232R) cBusGPIOIn(n int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmask := uint8(0x10 << uint(n))
	if f.cbusnibble&fmask == 0 {
		// Already input.
		return nil
	}
	v := f.cbusnibble &^ fmask
	if err := f.h.SetBitMode(v, bitModeCbusBitbang); err != nil {
		return err
	}
	f.cbusnibble = v
	return nil
}

// cBusGPIORead implements cBusGPIO.
func (f *FT232R) cBusGPIORead(n int) gpio.Level {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cBusReadLocked(n)
}

func (f *FT232R) cBusReadLocked(n int) gpio.Level {
	v, err := f.h.GetBitMode()
	if err != nil {
		return gpio.Low
	}
	f.cbusnibble = v
	vmask := uint8(1 << uint(n))
	return f.cbusnibble&vmask != 0
}

// cBusGPIOOut implements cBusGPIO.
func (f *FT232R) cBusGPIOOut(n int, l gpio.Level) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmask := uint8(0x10 << uint(n))
	vmask := uint8(1 << uint(n))
	v := f.cbusnibble | fmask
	if l {
		v |= vmask
	} else {
		v &^= vmask
	}
	if f.cbusnibble == v {
		// Was already in the right mode.
		return nil
	}
	if err := f.h.SetBitMode(v, bitModeCbusBitbang); err != nil {
		return err
	}
	f.cbusnibble = v
	return nil
}

//

var _ conn.Resource = Dev(nil)
