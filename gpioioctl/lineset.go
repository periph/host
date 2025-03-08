package gpioioctl

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/pin"
	"sync"
	"time"
)

// LineConfigOverride is an override for a LineSet configuration.
// For example, using this, you could configure a LineSet with
// multiple output lines, and a single input line with edge
// detection.
type LineConfigOverride struct {
	Lines     []string
	Direction LineDir
	Edge      gpio.Edge
	Pull      gpio.Pull
}

// LineSetConfig is used to create a structure for a LineSet request.
// It allows you to specify the default configuration for lines, as well
// as provide overrides for specific lines within the set.
type LineSetConfig struct {
	Lines            []string
	DefaultDirection LineDir
	DefaultEdge      gpio.Edge
	DefaultPull      gpio.Pull
	Overrides        []*LineConfigOverride
}

// AddOverrides adds a set of override values for specified lines. If a line
// specified is not already part of the configuration line set, it's dynamically
// added.
func (cfg *LineSetConfig) AddOverrides(direction LineDir, edge gpio.Edge, pull gpio.Pull, lines ...string) error {
	if len(cfg.Overrides) == _GPIO_V2_LINE_NUM_ATTRS_MAX {
		return fmt.Errorf("a maximum of %d override entries can be configured", _GPIO_V2_LINE_NUM_ATTRS_MAX)
	}
	for _, l := range lines {
		if cfg.getLineOffset(l) < 0 {
			cfg.Lines = append(cfg.Lines, l)
		}
	}
	cfg.Overrides = append(cfg.Overrides, &LineConfigOverride{Lines: lines, Direction: direction, Edge: edge, Pull: pull})
	return nil
}

func (cfg *LineSetConfig) getLineOffset(lineName string) int {
	for ix, name := range cfg.Lines {
		if name == lineName {
			return ix
		}
	}
	return -1
}

// Return a gpio_v2_line_request that represents this LineSetConfig.
// the returned value can then be used to request the lines.
func (cfg *LineSetConfig) getLineSetRequestStruct(lineNumbers []uint32) *gpio_v2_line_request {

	var lr gpio_v2_line_request
	for ix, char := range []byte(consumer) {
		lr.consumer[ix] = char
	}
	for ix, lineNumber := range lineNumbers {
		lr.setLineNumber(ix, lineNumber)
	}
	lr.num_lines = uint32(len(cfg.Lines))
	lr.config.flags = getFlags(cfg.DefaultDirection, cfg.DefaultEdge, cfg.DefaultPull)
	for _, lco := range cfg.Overrides {
		var mask uint64
		attr := gpio_v2_line_attribute{id: _GPIO_V2_LINE_ATTR_ID_FLAGS, value: getFlags(lco.Direction, lco.Edge, lco.Pull)}
		for _, line := range lco.Lines {
			offset := cfg.getLineOffset(line)
			mask |= uint64(1 << offset)

		}
		lr.config.attrs[lr.config.num_attrs] = gpio_v2_line_config_attribute{attr: attr, mask: mask}
		lr.config.num_attrs += 1
	}

	return &lr
}

// LineSet is a set of GPIO lines that can be manipulated as one device.
// A LineSet is created by calling GPIOChip.LineSet().  Using a LineSet,
// you can write to multiple pins, or read from multiple
// pins as one operation. Additionally, you can configure multiple lines
// for edge detection, and have a single WaitForEdge() call that will
// trigger on a change to any of the lines in the set. According
// to the Linux kernel docs:
//
// "A number of lines may be requested in the one line request, and request
// operations are performed on the requested lines by the kernel as
// atomically as possible. e.g. GPIO_V2_LINE_GET_VALUES_IOCTL will read all
// the requested lines at once."
//
// https://docs.kernel.org/userspace-api/gpio/gpio-v2-get-line-ioctl.html
type LineSet struct {
	lines []*LineSetLine
	mu    sync.Mutex
	// The anonymous file descriptor for this set of lines.
	fd int32
	// The file required for edge detection.
	fEdge *os.File
}

// Close the anonymous file descriptor allocated for this LineSet and release
// the pins.
func (ls *LineSet) Close() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.fd == 0 {
		return nil
	}
	var err error
	if ls.fEdge != nil {
		err = ls.fEdge.Close()
	} else if ls.fd != 0 {
		err = syscall_close_wrapper(int(ls.fd))
	}
	ls.fd = 0
	ls.fEdge = nil
	// TODO: This really needs erased from GPIOChip.LineSets
	return err
}

// LineCount returns the number of lines in this LineSet.
func (ls *LineSet) LineCount() int {
	return len(ls.lines)
}

// Lines returns the set of LineSetLine that are in
// this set.
func (ls *LineSet) Lines() []*LineSetLine {
	return ls.lines
}

func (ls *LineSet) Pins() []pin.Pin {
	pins := make([]pin.Pin, len(ls.lines))
	for ix, l := range ls.lines {
		pins[ix] = l
	}
	return pins
}

// Interrupt any calls to WaitForEdge().
func (ls *LineSet) Halt() error {
	if ls.fEdge != nil {
		return ls.fEdge.SetReadDeadline(time.UnixMilli(0))
	}
	return nil

}

// Out writes the set of bits to the LineSet's lines. If mask is 0, then the
// default mask of all bits is used. Note that by using the mask value,
// you can write to a subset of the lines if desired.
//
// bits is the values for each line in the bit set.
//
// mask is a bitmask indicating which bits should be applied.
func (ls *LineSet) Out(bits, mask gpio.GPIOValue) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	var data gpio_v2_line_values
	data.bits = uint64(bits)
	if mask == 0 {
		mask = (1 << ls.LineCount()) - 1
	}
	data.mask = uint64(mask)
	return ioctl_set_gpio_v2_line_values(uintptr(ls.fd), &data)
}

// Read the pins in this LineSet. This is done as one syscall to the
// operating system and will be very fast. mask is a bitmask of set pins
// to read. If 0, then all pins are read.
func (ls *LineSet) Read(mask gpio.GPIOValue) (gpio.GPIOValue, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if mask == 0 {
		mask = (1 << ls.LineCount()) - 1
	}
	var lvalues gpio_v2_line_values
	lvalues.mask = uint64(mask)
	if err := ioctl_get_gpio_v2_line_values(uintptr(ls.fd), &lvalues); err != nil {
		return 0, err
	}
	return gpio.GPIOValue(lvalues.bits), nil
}

func (ls *LineSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Lines []*LineSetLine `json:"Lines"`
	}{
		Lines: ls.lines})
}

// String returns the LineSet information in JSON, along with the details for
// all of the lines.
func (ls *LineSet) String() string {
	json, _ := json.MarshalIndent(ls, "", "    ")
	return string(json)
}

// WaitForEdge waits for an edge to be triggered on the LineSet.
//
// Returns:
//
// number - the number of the line that was triggered.
//
// edge - The edge value. gpio.Edge. If a timeout or halt occurred,
// then the edge returned will be gpio.NoEdge
//
// err - Error value if any.
func (ls *LineSet) WaitForEdge(timeout time.Duration) (number int, edge gpio.Edge, err error) {
	number = 0
	edge = gpio.NoEdge
	if ls.fEdge == nil {
		err = syscall_nonblock_wrapper(int(ls.fd), true)
		if err != nil {
			err = fmt.Errorf("WaitForEdge() - SetNonblock: %w", err)
			return
		}
		ls.fEdge = os.NewFile(uintptr(ls.fd), "gpio-lineset")
	}

	if timeout == 0 {
		err = ls.fEdge.SetReadDeadline(time.Time{})
	} else {
		err = ls.fEdge.SetReadDeadline(time.Now().Add(timeout))
	}
	if err != nil {
		err = fmt.Errorf("WaitForEdge() - SetReadDeadline(): %w", err)
		return
	}

	var event gpio_v2_line_event
	err = binary.Read(ls.fEdge, binary.LittleEndian, &event)
	if err != nil {
		return
	}
	if event.Id == _GPIO_V2_LINE_EVENT_RISING_EDGE {
		edge = gpio.RisingEdge
	} else if event.Id == _GPIO_V2_LINE_EVENT_FALLING_EDGE {
		edge = gpio.FallingEdge
	}
	number = int(event.Offset)
	return
}

// ByOffset returns a line by it's offset in the LineSet.  See ByName() for an
// example that casts the return value to a LineSetLine
func (ls *LineSet) ByOffset(offset int) pin.Pin {
	if offset < 0 || offset >= len(ls.lines) {
		return nil
	}
	return ls.lines[offset]
}

// ByName returns a Line by name from the LineSet. To cast the returned value
// to a LineSet line, use:
//
//	var lsl *gpioioctl.LineSetLine
//	lsl, ok := ls.ByNumber(line0.Number()).(*gpioioctl.LineSetLine)
//	if !ok {
//	  log.Fatal("error converting to LineSetLine")
//	}
func (ls *LineSet) ByName(name string) pin.Pin {
	for _, line := range ls.lines {
		if line.Name() == name {
			return line
		}
	}
	return nil
}

// LineNumber Return a line from the LineSet via it's GPIO line
// number. See ByName() for an example that casts the return value to a
// LineSetLine
func (ls *LineSet) ByNumber(number int) pin.Pin {
	for _, line := range ls.lines {
		if line.Number() == number {
			return line
		}
	}
	return nil
}

// LineSetLine is a specific line in a lineset. Using a LineSetLine,
// you can read/write to a single pin in the set using the PinIO
// interface.
type LineSetLine struct {
	// The GPIO Line Number
	number uint32
	// The offset for this LineSet struct
	offset    uint32
	name      string
	parent    *LineSet
	direction LineDir
	pull      gpio.Pull
	edge      gpio.Edge
}

/*
   gpio.Pin
*/

// Number returns the Line's GPIO Line Number. Implements gpio.Pin
func (lsl *LineSetLine) Number() int {
	return int(lsl.number)
}

// Name returns the line's name. Implements gpio.Pin
func (lsl *LineSetLine) Name() string {
	return lsl.name
}

func (lsl *LineSetLine) Function() string {
	return "not implemented"
}

func (lsl *LineSetLine) Direction() LineDir {
	return lsl.direction
}

func (lsl *LineSetLine) Edge() gpio.Edge {
	return lsl.edge
}

// Out writes to this specific GPIO line.
func (lsl *LineSetLine) Out(l gpio.Level) error {
	var mask, bits gpio.GPIOValue
	mask = 1 << lsl.offset
	if l {
		bits |= mask
	}
	return lsl.parent.Out(bits, mask)
}

// PWM is not implemented because of kernel design.
func (lsl *LineSetLine) PWM(gpio.Duty, physic.Frequency) error {
	return errors.New("not implemented")
}

// Halt interrupts a pending WaitForEdge. You can't halt a read
// for a single line in a LineSet, so this returns an error. Use
// LineSet.Halt()
func (lsl *LineSetLine) Halt() error {
	return errors.New("you can't halt an individual line in a LineSet. you must halt the LineSet")
}

// In configures the line for input. Since individual lines in a
// LineSet cannot be re-configured this always returns an error.
func (lsl *LineSetLine) In(pull gpio.Pull, edge gpio.Edge) error {
	return errors.New("a LineSet line cannot be re-configured")
}

// Read returns the value of this specific line.
func (lsl *LineSetLine) Read() gpio.Level {
	var mask gpio.GPIOValue = 1 << lsl.offset
	bits, err := lsl.parent.Read(mask)
	if err != nil {
		log.Printf("LineSetLine.Read() Error reading line %d. Error: %s\n", lsl.number, err)
		return false
	}

	return (bits & mask) == mask
}

func (lsl *LineSetLine) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name      string `json:"Name"`
		Offset    uint32 `json:"Offset"`
		Number    int    `json:"Number"`
		Direction Label  `json:"Direction"`
		Pull      Label  `json:"Pull"`
		Edges     Label  `json:"Edges"`
	}{
		Name:      lsl.Name(),
		Offset:    lsl.Offset(),
		Number:    lsl.Number(),
		Direction: DirectionLabels[lsl.direction],
		Pull:      PullLabels[lsl.pull],
		Edges:     EdgeLabels[lsl.edge]})
}

// String returns information about the line in JSON format.
func (lsl *LineSetLine) String() string {
	json, _ := json.MarshalIndent(lsl, "", "    ")
	return string(json)
}

// WaitForEdge will always return false for a LineSetLine. You MUST
// use LineSet.WaitForEdge()
func (lsl *LineSetLine) WaitForEdge(timeout time.Duration) bool {
	return false
}

// Pull returns the configured PullUp/PullDown value for this line.
func (lsl *LineSetLine) Pull() gpio.Pull {
	return lsl.pull
}

// DefaultPull return gpio.PullNoChange.
//
// The GPIO v2 ioctls do not support this.
func (lsl *LineSetLine) DefaultPull() gpio.Pull {
	return gpio.PullNoChange
}

// Offset returns the offset if this LineSetLine within the LineSet.
// 0..LineSet.LineCount
func (lsl *LineSetLine) Offset() uint32 {
	return lsl.offset
}

// Ensure that Interfaces for these types are implemented fully.
var _ gpio.Group = &LineSet{}
var _ gpio.PinIO = &LineSetLine{}
var _ gpio.PinIn = &LineSetLine{}
var _ gpio.PinOut = &LineSetLine{}
