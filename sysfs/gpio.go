// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package sysfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/pin"
	"periph.io/x/host/v3/distro"
	"periph.io/x/host/v3/fs"
)

// Pins is all the pins exported by GPIO sysfs.
//
// Some CPU architectures have the pin numbers start at 0 and use consecutive
// pin numbers but this is not the case for all CPU architectures, some
// have gaps in the pin numbering.
//
// This global variable is initialized once at driver initialization and isn't
// mutated afterward. Do not modify it.
var Pins map[int]*Pin

// Pin represents one GPIO pin as found by sysfs.
type Pin struct {
	number int
	name   string
	root   string // Something like /sys/class/gpio/gpio%d/

	mu         sync.Mutex
	err        error     // If open() failed
	direction  direction // Cache of the last known direction
	edge       gpio.Edge // Cache of the last edge used.
	fDirection fileIO    // handle to /sys/class/gpio/gpio*/direction; never closed
	fEdge      fileIO    // handle to /sys/class/gpio/gpio*/edge; never closed
	fValue     fileIO    // handle to /sys/class/gpio/gpio*/value; never closed
	event      fs.Event  // Initialized once
	buf        [4]byte   // scratch buffer for Func(), Read() and Out()
}

// String implements conn.Resource.
func (p *Pin) String() string {
	return p.name
}

// Halt implements conn.Resource.
//
// It stops edge detection if enabled.
func (p *Pin) Halt() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.haltEdge()
}

// Name implements pin.Pin.
func (p *Pin) Name() string {
	return p.name
}

// Number implements pin.Pin.
func (p *Pin) Number() int {
	return p.number
}

// Function implements pin.Pin.
func (p *Pin) Function() string {
	return string(p.Func())
}

// Func implements pin.PinFunc.
func (p *Pin) Func() pin.Func {
	p.mu.Lock()
	defer p.mu.Unlock()
	// TODO(maruel): There's an internal bug which causes p.direction to be
	// invalid (!?) Need to figure it out ASAP.
	if err := p.open(); err != nil {
		return pin.FuncNone
	}
	if _, err := seekRead(p.fDirection, p.buf[:]); err != nil {
		return pin.FuncNone
	}
	if p.buf[0] == 'i' && p.buf[1] == 'n' {
		p.direction = dIn
	} else if p.buf[0] == 'o' && p.buf[1] == 'u' && p.buf[2] == 't' {
		p.direction = dOut
	}
	if p.direction == dIn {
		if p.Read() {
			return gpio.IN_HIGH
		}
		return gpio.IN_LOW
	} else if p.direction == dOut {
		if p.Read() {
			return gpio.OUT_HIGH
		}
		return gpio.OUT_LOW
	}
	return pin.FuncNone
}

// SupportedFuncs implements pin.PinFunc.
func (p *Pin) SupportedFuncs() []pin.Func {
	return []pin.Func{gpio.IN, gpio.OUT}
}

// SetFunc implements pin.PinFunc.
func (p *Pin) SetFunc(f pin.Func) error {
	switch f {
	case gpio.IN:
		return p.In(gpio.PullNoChange, gpio.NoEdge)
	case gpio.OUT_HIGH:
		return p.Out(gpio.High)
	case gpio.OUT, gpio.OUT_LOW:
		return p.Out(gpio.Low)
	default:
		return p.wrap(errors.New("unsupported function"))
	}
}

// In implements gpio.PinIn.
func (p *Pin) In(pull gpio.Pull, edge gpio.Edge) error {
	if pull != gpio.PullNoChange && pull != gpio.Float {
		return p.wrap(errors.New("doesn't support pull-up/pull-down"))
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.direction != dIn {
		if err := p.open(); err != nil {
			return p.wrap(err)
		}
		if err := seekWrite(p.fDirection, bIn); err != nil {
			return p.wrap(err)
		}
		p.direction = dIn
	}
	// Always push none to help accumulated flush edges. This is not fool proof
	// but it seems to help.
	if p.fEdge != nil {
		if err := seekWrite(p.fEdge, bNone); err != nil {
			return p.wrap(err)
		}
	}
	// Assume that when the pin was switched, the driver doesn't recall if edge
	// triggering was enabled.
	if edge != gpio.NoEdge {
		if p.fEdge == nil {
			var err error
			p.fEdge, err = fileIOOpen(p.root+"edge", os.O_RDWR)
			if err != nil {
				return p.wrap(err)
			}
			if err = p.event.MakeEvent(p.fValue.Fd()); err != nil {
				_ = p.fEdge.Close()
				p.fEdge = nil
				return p.wrap(err)
			}
		}
		// Always reset the edge detection mode to none after starting the epoll
		// otherwise edges are not always delivered, as observed on an Allwinner A20
		// running kernel 4.14.14.
		if err := seekWrite(p.fEdge, bNone); err != nil {
			return p.wrap(err)
		}
		var b []byte
		switch edge {
		case gpio.RisingEdge:
			b = bRising
		case gpio.FallingEdge:
			b = bFalling
		case gpio.BothEdges:
			b = bBoth
		}
		if err := seekWrite(p.fEdge, b); err != nil {
			return p.wrap(err)
		}
	}
	p.edge = edge
	// This helps to remove accumulated edges but this is not 100% sufficient.
	// Most of the time the interrupts are handled promptly enough that this loop
	// flushes the accumulated interrupt.
	// Sometimes the kernel may have accumulated interrupts that haven't been
	// processed for a long time, it can easily be >300µs even on a quite idle
	// CPU. In this case, the loop below is not sufficient, since the interrupt
	// will happen afterward "out of the blue".
	if edge != gpio.NoEdge {
		p.WaitForEdge(0)
	}
	return nil
}

// Read implements gpio.PinIn.
func (p *Pin) Read() gpio.Level {
	// There's no lock here.
	if p.fValue == nil {
		return gpio.Low
	}
	if _, err := seekRead(p.fValue, p.buf[:]); err != nil {
		// Error.
		return gpio.Low
	}
	if p.buf[0] == '0' {
		return gpio.Low
	}
	if p.buf[0] == '1' {
		return gpio.High
	}
	// Error.
	return gpio.Low
}

// WaitForEdge implements gpio.PinIn.
func (p *Pin) WaitForEdge(timeout time.Duration) bool {
	// Run lockless, as the normal use is to call in a busy loop.
	var ms int
	if timeout == -1 {
		ms = -1
	} else {
		ms = int(timeout / time.Millisecond)
	}
	start := time.Now()
	for {
		if nr, err := p.event.Wait(ms); err != nil {
			return false
		} else if nr == 1 {
			// TODO(maruel): According to pigpio, the correct way to consume the
			// interrupt is to call Seek().
			return true
		}
		// A signal occurred.
		if timeout != -1 {
			ms = int((timeout - time.Since(start)) / time.Millisecond)
		}
		if ms <= 0 {
			return false
		}
	}
}

// Pull implements gpio.PinIn.
//
// It returns gpio.PullNoChange since gpio sysfs has no support for input pull
// resistor.
func (p *Pin) Pull() gpio.Pull {
	return gpio.PullNoChange
}

// DefaultPull implements gpio.PinIn.
//
// It returns gpio.PullNoChange since gpio sysfs has no support for input pull
// resistor.
func (p *Pin) DefaultPull() gpio.Pull {
	return gpio.PullNoChange
}

// Out implements gpio.PinOut.
func (p *Pin) Out(l gpio.Level) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.direction != dOut {
		if err := p.open(); err != nil {
			return p.wrap(err)
		}
		if err := p.haltEdge(); err != nil {
			return err
		}
		// "To ensure glitch free operation, values "low" and "high" may be written
		// to configure the GPIO as an output with that initial value."
		var d []byte
		if l == gpio.Low {
			d = bLow
		} else {
			d = bHigh
		}
		if err := seekWrite(p.fDirection, d); err != nil {
			return p.wrap(err)
		}
		p.direction = dOut
		return nil
	}
	if l == gpio.Low {
		p.buf[0] = '0'
	} else {
		p.buf[0] = '1'
	}
	if err := seekWrite(p.fValue, p.buf[:1]); err != nil {
		return p.wrap(err)
	}
	return nil
}

// PWM implements gpio.PinOut.
//
// This is not supported on sysfs.
func (p *Pin) PWM(gpio.Duty, physic.Frequency) error {
	return p.wrap(errors.New("pwm is not supported via sysfs"))
}

//

// open opens the gpio sysfs handle to /value and /direction.
//
// lock must be held.
func (p *Pin) open() error {
	if p.fDirection != nil || p.err != nil {
		return p.err
	}

	if drvGPIO.exportHandle == nil {
		return errors.New("sysfs gpio is not initialized")
	}

	// Try to open the pin if it was there. It's possible it had been exported
	// already.
	if p.fValue, p.err = fileIOOpen(p.root+"value", os.O_RDWR); p.err == nil {
		// Fast track.
		goto direction
	} else if !os.IsNotExist(p.err) {
		// It exists but not accessible, not worth doing the remainder.
		p.err = fmt.Errorf("need more access, try as root or setup udev rules: %v", p.err)
		return p.err
	}

	if _, p.err = drvGPIO.exportHandle.Write([]byte(strconv.Itoa(p.number))); p.err != nil && !isErrBusy(p.err) {
		if os.IsPermission(p.err) {
			p.err = fmt.Errorf("need more access, try as root or setup udev rules: %v", p.err)
		}
		return p.err
	}

	// There's a race condition where the file may be created but udev is still
	// running the Raspbian udev rule to make it readable to the current user.
	// It's simpler to just loop a little as if /export is accessible, it doesn't
	// make sense that gpioN/value doesn't become accessible eventually.
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		// The virtual file creation is synchronous when writing to /export; albeit
		// udev rule execution is asynchronous, so file mode change via udev rules
		// takes some time to propagate.
		if p.fValue, p.err = fileIOOpen(p.root+"value", os.O_RDWR); p.err == nil || !os.IsPermission(p.err) {
			// Either success or a failure that is not a permission error.
			break
		}
	}
	if p.err != nil {
		return p.err
	}

direction:
	if p.fDirection, p.err = fileIOOpen(p.root+"direction", os.O_RDWR); p.err != nil {
		_ = p.fValue.Close()
		p.fValue = nil
	}
	return p.err
}

// haltEdge stops any on-going edge detection.
func (p *Pin) haltEdge() error {
	if p.edge != gpio.NoEdge {
		if err := seekWrite(p.fEdge, bNone); err != nil {
			return p.wrap(err)
		}
		p.edge = gpio.NoEdge
		// This is still important to remove an accumulated edge.
		p.WaitForEdge(0)
	}
	return nil
}

func (p *Pin) wrap(err error) error {
	return fmt.Errorf("sysfs-gpio (%s): %v", p, err)
}

//

type direction int

const (
	dUnknown direction = 0
	dIn      direction = 1
	dOut     direction = 2
)

var (
	bIn      = []byte("in")
	bLow     = []byte("low")
	bHigh    = []byte("high")
	bNone    = []byte("none")
	bRising  = []byte("rising")
	bFalling = []byte("falling")
	bBoth    = []byte("both")
)

// readInt reads a pseudo-file (sysfs) that is known to contain an integer and
// returns the parsed number.
func readInt(path string) (int, error) {
	f, err := fileIOOpen(path, os.O_RDONLY)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var b [24]byte
	n, err := f.Read(b[:])
	if err != nil {
		return 0, err
	}
	raw := b[:n]
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return 0, errors.New("invalid value")
	}
	return strconv.Atoi(string(raw[:len(raw)-1]))
}

// driverGPIO implements periph.Driver.
type driverGPIO struct {
	exportHandle io.Writer // handle to /sys/class/gpio/export
}

func (d *driverGPIO) String() string {
	return "sysfs-gpio"
}

func (d *driverGPIO) Prerequisites() []string {
	return nil
}

func (d *driverGPIO) After() []string {
	return nil
}

// Init initializes GPIO sysfs handling code.
//
// Uses gpio sysfs as described at
// https://www.kernel.org/doc/Documentation/gpio/sysfs.txt
//
// GPIO sysfs is often the only way to do edge triggered interrupts. Doing this
// requires cooperation from a driver in the kernel.
//
// The main drawback of GPIO sysfs is that it doesn't expose internal pull
// resistor and it is much slower than using memory mapped hardware registers.
func (d *driverGPIO) Init() (bool, error) {
	items, err := filepath.Glob("/sys/class/gpio/gpiochip*")
	if err != nil {
		return true, err
	}
	if len(items) == 0 {
		return false, errors.New("no GPIO pin found")
	}

	// There are hosts that use non-continuous pin numbering so use a map instead
	// of an array.
	Pins = map[int]*Pin{}
	for _, item := range items {
		if err = d.parseGPIOChip(item + "/"); err != nil {
			return true, err
		}
	}
	drvGPIO.exportHandle, err = fileIOOpen("/sys/class/gpio/export", os.O_WRONLY)
	if os.IsPermission(err) {
		return true, fmt.Errorf("need more access, try as root or setup udev rules: %v", err)
	}
	return true, err
}

func (d *driverGPIO) parseGPIOChip(path string) error {
	base, err := readInt(path + "base")
	if err != nil {
		return err
	}
	number, err := readInt(path + "ngpio")
	if err != nil {
		return err
	}

	boardModel := distro.DTModel()

	// TODO(maruel): The chip driver may lie and lists GPIO pins that cannot be
	// exported. The only way to know about it is to export it before opening.
	for i := base; i < base+number; i++ {
		if _, ok := Pins[i]; ok {
			return fmt.Errorf("found two pins with number %d", i)
		}
		p := &Pin{
			number: i,
			name:   fmt.Sprintf("GPIO%d", i),
			root:   getSymlinkRoot(boardModel, i),
		}
		Pins[i] = p
		if err := gpioreg.Register(p); err != nil {
			return err
		}
		// If there is a CPU memory mapped gpio pin with the same number, the
		// driver has to unregister this pin and map its own after.
		if err := gpioreg.RegisterAlias(strconv.Itoa(i), p.name); err != nil {
			return err
		}
	}
	return nil
}

const jetsonOrinAgxOffset = 316

// The NVidia Jetson Orin AGX uses nonstandard names within /sys/class/gpio. This is a mapping
// from pin numbers starting at the offset above to their names on that machine. It should be
// considered immutable.
var jetsonOrinAgxPinNames = [196]string{
	"AA.00", "AA.01", "AA.02", "AA.03", "AA.04", "AA.05", "AA.06", "AA.07", "BB.00", "BB.01",
	"BB.02", "BB.03", "CC.00", "CC.01", "CC.02", "CC.03", "CC.04", "CC.05", "CC.06", "CC.07",
	"DD.00", "DD.01", "DD.02", "EE.00", "EE.01", "EE.02", "EE.03", "EE.04", "EE.05", "EE.06",
	"EE.07", "GG.00", "A.00", "A.01", "A.02", "A.03", "A.04", "A.05", "A.06", "A.07",
	"B.00", "C.00", "C.01", "C.02", "C.03", "C.04", "C.05", "C.06", "C.07", "D.00",
	"D.01", "D.02", "D.03", "E.00", "E.01", "E.02", "E.03", "E.04", "E.05", "E.06",
	"E.07", "F.00", "F.01", "F.02", "F.03", "F.04", "F.05", "G.00", "G.01", "G.02",
	"G.03", "G.04", "G.05", "G.06", "G.07", "H.00", "H.01", "H.02", "H.03", "H.04",
	"H.05", "H.06", "H.07", "I.00", "I.01", "I.02", "I.03", "I.04", "I.05", "I.06",
	"J.00", "J.01", "J.02", "J.03", "J.04", "J.05", "K.00", "K.01", "K.02", "K.03",
	"K.04", "K.05", "K.06", "K.07", "L.00", "L.01", "L.02", "L.03", "M.00", "M.01",
	"M.02", "M.03", "M.04", "M.05", "M.06", "M.07", "N.00", "N.01", "N.02", "N.03",
	"N.04", "N.05", "N.06", "N.07", "P.00", "P.01", "P.02", "P.03", "P.04", "P.05",
	"P.06", "P.07", "Q.00", "Q.01", "Q.02", "Q.03", "Q.04", "Q.05", "Q.06", "Q.07",
	"R.00", "R.01", "R.02", "R.03", "R.04", "R.05", "X.00", "X.01", "X.02", "X.03",
	"X.04", "X.05", "X.06", "X.07", "Y.00", "Y.01", "Y.02", "Y.03", "Y.04", "Y.05",
	"Y.06", "Y.07", "Z.00", "Z.01", "Z.02", "Z.03", "Z.04", "Z.05", "Z.06", "Z.07",
	"AC.00", "AC.01", "AC.02", "AC.03", "AC.04", "AC.05", "AC.06", "AC.07", "AD.00", "AD.01",
	"AD.02", "AD.03", "AE.00", "AE.01", "AF.00", "AF.01", "AF.02", "AF.03", "AG.00", "AG.01",
	"AG.02", "AG.03", "AG.04", "AG.05", "AG.06", "AG.07",
}

func getSymlinkRoot(boardModel string, pinNumber int) string {
	if boardModel == "Jetson AGX Orin" {
		pinName := jetsonOrinAgxPinNames[pinNumber-jetsonOrinAgxOffset]
		return fmt.Sprintf("/sys/class/gpio/P%s/", pinName)
	}

	// Nearly all boards use this naming scheme:
	return fmt.Sprintf("/sys/class/gpio/gpio%d/", pinNumber)
}

func init() {
	if isLinux {
		driverreg.MustRegister(&drvGPIO)
	}
}

var drvGPIO driverGPIO

var _ conn.Resource = &Pin{}
var _ gpio.PinIn = &Pin{}
var _ gpio.PinOut = &Pin{}
var _ gpio.PinIO = &Pin{}
var _ pin.PinFunc = &Pin{}
