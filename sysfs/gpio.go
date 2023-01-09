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
	// TODO(maruel): The chip driver may lie and lists GPIO pins that cannot be
	// exported. The only way to know about it is to export it before opening.
	for i := base; i < base+number; i++ {
		if _, ok := Pins[i]; ok {
			return fmt.Errorf("found two pins with number %d", i)
		}
		p := &Pin{
			number: i,
			name:   fmt.Sprintf("GPIO%d", i),
			root:   getSymlinkRoot(i),
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

func getSymlinkRoot(pinNumber int) string {
	// The NVidia Jetson Orin AGX uses nonstandard names within /sys/class/gpio. This is a mapping
	// from pin numbers to their names on that machine.
	jetsonOrinAgxPins := map[int]string{
		316: "AA.00", 317: "AA.01", 318: "AA.02", 319: "AA.03", 320: "AA.04", 321: "AA.05",
		322: "AA.06", 323: "AA.07", 324: "BB.00", 325: "BB.01", 326: "BB.02", 327: "BB.03",
		328: "CC.00", 329: "CC.01", 330: "CC.02", 331: "CC.03", 332: "CC.04", 333: "CC.05",
		334: "CC.06", 335: "CC.07", 336: "DD.00", 337: "DD.01", 338: "DD.02", 339: "EE.00",
		340: "EE.01", 341: "EE.02", 342: "EE.03", 343: "EE.04", 344: "EE.05", 345: "EE.06",
		346: "EE.07", 347: "GG.00", 348: "A.00",  349: "A.01",  350: "A.02",  351: "A.03",
		352: "A.04",  353: "A.05",  354: "A.06",  355: "A.07",  356: "B.00",  357: "C.00",
		358: "C.01",  359: "C.02",  360: "C.03",  361: "C.04",  362: "C.05",  363: "C.06",
		364: "C.07",  365: "D.00",  366: "D.01",  367: "D.02",  368: "D.03",  369: "E.00",
		370: "E.01",  371: "E.02",  372: "E.03",  373: "E.04",  374: "E.05",  375: "E.06",
		376: "E.07",  377: "F.00",  378: "F.01",  379: "F.02",  380: "F.03",  381: "F.04",
		382: "F.05",  383: "G.00",  384: "G.01",  385: "G.02",  386: "G.03",  387: "G.04",
		388: "G.05",  389: "G.06",  390: "G.07",  391: "H.00",  392: "H.01",  393: "H.02",
		394: "H.03",  395: "H.04",  396: "H.05",  397: "H.06",  398: "H.07",  399: "I.00",
		400: "I.01",  401: "I.02",  402: "I.03",  403: "I.04",  404: "I.05",  405: "I.06",
		406: "J.00",  407: "J.01",  408: "J.02",  409: "J.03",  410: "J.04",  411: "J.05",
		412: "K.00",  413: "K.01",  414: "K.02",  415: "K.03",  416: "K.04",  417: "K.05",
		418: "K.06",  419: "K.07",  420: "L.00",  421: "L.01",  422: "L.02",  423: "L.03",
		424: "M.00",  425: "M.01",  426: "M.02",  427: "M.03",  428: "M.04",  429: "M.05",
		430: "M.06",  431: "M.07",  432: "N.00",  433: "N.01",  434: "N.02",  435: "N.03",
		436: "N.04",  437: "N.05",  438: "N.06",  439: "N.07",  440: "P.00",  441: "P.01",
		442: "P.02",  443: "P.03",  444: "P.04",  445: "P.05",  446: "P.06",  447: "P.07",
		448: "Q.00",  449: "Q.01",  450: "Q.02",  451: "Q.03",  452: "Q.04",  453: "Q.05",
		454: "Q.06",  455: "Q.07",  456: "R.00",  457: "R.01",  458: "R.02",  459: "R.03",
		460: "R.04",  461: "R.05",  462: "X.00",  463: "X.01",  464: "X.02",  465: "X.03",
		466: "X.04",  467: "X.05",  468: "X.06",  469: "X.07",  470: "Y.00",  471: "Y.01",
		472: "Y.02",  473: "Y.03",  474: "Y.04",  475: "Y.05",  476: "Y.06",  477: "Y.07",
		478: "Z.00",  479: "Z.01",  480: "Z.02",  481: "Z.03",  482: "Z.04",  483: "Z.05",
		484: "Z.06",  485: "Z.07",  486: "AC.00", 487: "AC.01", 488: "AC.02", 489: "AC.03",
		490: "AC.04", 491: "AC.05", 492: "AC.06", 493: "AC.07", 494: "AD.00", 495: "AD.01",
		496: "AD.02", 497: "AD.03", 498: "AE.00", 499: "AE.01", 500: "AF.00", 501: "AF.01",
		502: "AF.02", 503: "AF.03", 504: "AG.00", 505: "AG.01", 506: "AG.02", 507: "AG.03",
		508: "AG.04", 509: "AG.05", 510: "AG.06", 511: "AG.07",
	}

	// Watch out! The board name likely ends in a null byte.
	if boardName == "Jetson AGX Orin\000" {
		val, ok := jetsonOrinAgxPins[pinNumber]
		if ok {
			return fmt.Sprintf("/sys/class/gpio/P%s/", val)
		}
		// For unknown pins, try the default name but expect failures regardless.
	}
	// Nearly all boards use this naming scheme:
	return fmt.Sprintf("/sys/class/gpio/gpio%d/", pinNumber)
}

var boardName string // The contents of /proc/device-tree/model, if it exists

func initBoardName() {
	f, err := fileIOOpen("/proc/device-tree/model", os.O_RDONLY)
	if err != nil {
		return // Unknown board, we'll try using the default pin names
	}
	defer f.Close()
	var b [1024]byte
	n, err := f.Read(b[:])
	if err != nil {
		return
	}
	boardName = string(b[:n])
}

func init() {
	initBoardName()
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
