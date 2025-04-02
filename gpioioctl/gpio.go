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
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/pin"
)

// LineDir is the configured direction of a GPIOLine.
type LineDir uint32

const (
	LineDirNotSet LineDir = 0
	LineInput     LineDir = 1
	LineOutput    LineDir = 2
)

// The consumer name to use for line requests. Initialized in init()
var consumer []byte

// The set of GPIO Chips found on the running device.
var Chips []*GPIOChip

type Label string

var DirectionLabels = []Label{"NotSet", "Input", "Output"}
var PullLabels = []Label{"PullNoChange", "Float", "PullDown", "PullUp"}
var EdgeLabels = []Label{"NoEdge", "RisingEdge", "FallingEdge", "BothEdges"}

// A GPIOLine represents a specific line of a GPIO Chip. GPIOLine implements
// periph.io/conn/v3/gpio.PinIn, PinIO, and PinOut. A line is obtained by
// calling gpioreg.ByName(), or using the GPIOChip.ByName() or ByNumber()
// methods.
type GPIOLine struct {
	// The Offset of this line on the chip. Note that this has NO RELATIONSHIP
	// to the pin numbering scheme that may be in use on a board.
	number uint32
	// The name supplied by the OS Driver
	name string
	// If the line is in use, this may be populated with the
	// consuming application's information.
	consumer  string
	edge      gpio.Edge
	pull      gpio.Pull
	direction LineDir
	mu        sync.Mutex
	chip_fd   uintptr
	fd        int32
	fEdge     *os.File
}

func newGPIOLine(lineNum uint32, name string, consumer string, fd uintptr) *GPIOLine {
	line := GPIOLine{
		number:   lineNum,
		name:     strings.Trim(name, "\x00"),
		consumer: strings.Trim(consumer, "\x00"),
		mu:       sync.Mutex{},
		chip_fd:  fd,
	}
	return &line
}

// Close the line, and any associated files/file descriptors that were created.
func (line *GPIOLine) Close() {
	line.mu.Lock()
	defer line.mu.Unlock()
	if line.fEdge != nil {
		_ = line.fEdge.Close()
	} else if line.fd != 0 {
		_ = syscall_close_wrapper(int(line.fd))
	}
	line.fd = 0
	line.consumer = ""
	line.edge = gpio.NoEdge
	line.direction = LineDirNotSet
	line.pull = gpio.PullNoChange
	line.fEdge = nil
}

// Consumer returns the name of the consumer specified for a line when
// a line request was performed. The format used by this library is
// program_name@pid.
func (line *GPIOLine) Consumer() string {
	return line.consumer
}

// DefaultPull - return gpio.PullNoChange. Reviewing the GPIO v2 Kernel IOCTL docs, this isn't possible.
func (line *GPIOLine) DefaultPull() gpio.Pull {
	return gpio.PullNoChange
}

// Halt interrupts a pending WaitForEdge() command.
func (line *GPIOLine) Halt() error {
	if line.fEdge != nil {
		return line.fEdge.SetReadDeadline(time.UnixMilli(0))
	}
	return nil
}

// Configure the GPIOLine for input. Implements gpio.PinIn.
func (line *GPIOLine) In(pull gpio.Pull, edge gpio.Edge) error {
	line.mu.Lock()
	defer line.mu.Unlock()
	flags := getFlags(LineInput, edge, pull)
	line.edge = edge
	line.direction = LineInput
	line.pull = pull

	return line.setLine(flags)
}

// Implements gpio.Pin
func (line *GPIOLine) Name() string {
	return line.name
}

// Number returns the line offset/number within the GPIOChip. Implements gpio.Pin
func (line *GPIOLine) Number() int {
	return int(line.number)
}

// Write the specified level to the line. Implements gpio.PinOut
func (line *GPIOLine) Out(l gpio.Level) error {
	line.mu.Lock()
	defer line.mu.Unlock()
	if line.direction != LineOutput {
		err := line.setOut()
		if err != nil {
			return fmt.Errorf("GPIOLine.Out(): %w", err)
		}
	}
	var data gpio_v2_line_values
	data.mask = 0x01
	if l {
		data.bits = 0x01
	}
	return ioctl_set_gpio_v2_line_values(uintptr(line.fd), &data)
}

// Pull returns the configured Line Bias.
func (line *GPIOLine) Pull() gpio.Pull {
	return line.pull
}

// Not implemented because the kernel PWM is not in the ioctl library
// but a different one.
func (line *GPIOLine) PWM(gpio.Duty, physic.Frequency) error {
	return errors.New("PWM() not implemented")
}

// Read the value of this line. Implements gpio.PinIn
func (line *GPIOLine) Read() gpio.Level {
	if line.direction != LineInput {
		err := line.In(gpio.PullUp, gpio.NoEdge)
		if err != nil {
			log.Println("GPIOLine.Read(): ", err)
			return false
		}
	}
	line.mu.Lock()
	defer line.mu.Unlock()
	var data gpio_v2_line_values
	data.mask = 0x01
	err := ioctl_get_gpio_v2_line_values(uintptr(line.fd), &data)
	if err != nil {
		log.Println(err)
		return false
	}
	return data.bits&0x01 == 0x01
}

func (line *GPIOLine) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Line      int    `json:"Line"`
		Name      string `json:"Name"`
		Consumer  string `json:"Consumer"`
		Direction Label  `json:"Direction"`
		Pull      Label  `json:"Pull"`
		Edges     Label  `json:"Edges"`
	}{
		Line:      line.Number(),
		Name:      line.Name(),
		Consumer:  line.Consumer(),
		Direction: DirectionLabels[line.direction],
		Pull:      PullLabels[line.pull],
		Edges:     EdgeLabels[line.edge]})
}

// String returns information about the line in valid JSON format.
func (line *GPIOLine) String() string {
	json, _ := json.MarshalIndent(line, "", "    ")
	return string(json)
}

// Wait for this line to trigger and edge event. You must call In() with
// a valid edge for this to work. To interrupt a waiting line, call Halt().
// Implements gpio.PinIn.
//
// Note that this does not return which edge was detected for the
// gpio.EdgeBoth configuration. If you really need the edge,
// LineSet.WaitForEdge() does return the edge that triggered.
//
// timeout for the edge change to occur. If 0, waits forever.
func (line *GPIOLine) WaitForEdge(timeout time.Duration) bool {
	if line.edge == gpio.NoEdge || line.direction == LineDirNotSet {
		log.Println("call to WaitForEdge() when line hasn't been configured for edge detection.")
		return false
	}
	var err error
	if line.fEdge == nil {
		err = syscall_nonblock_wrapper(int(line.fd), true)
		if err != nil {
			log.Println("WaitForEdge() SetNonblock(): ", err)
			return false
		}
		line.fEdge = os.NewFile(uintptr(line.fd), fmt.Sprintf("gpio-%d", line.number))
	}

	if timeout == 0 {
		err = line.fEdge.SetReadDeadline(time.Time{})
	} else {
		err = line.fEdge.SetReadDeadline(time.Now().Add(timeout))
	}
	if err != nil {
		log.Println("GPIOLine.WaitForEdge() setReadDeadline() returned:", err)
		return false
	}
	var event gpio_v2_line_event
	// If the read times out, or is interrupted via Halt(), it will
	// return "i/o timeout"
	err = binary.Read(line.fEdge, binary.LittleEndian, &event)

	return err == nil
}

// Return the file descriptor associated with this line. If it
// hasn't been previously requested, then open the file descriptor
// for it.
func (line *GPIOLine) getLine() (int32, error) {
	if line.fd != 0 {
		return line.fd, nil
	}
	var req gpio_v2_line_request
	req.offsets[0] = uint32(line.number)
	req.num_lines = 1
	for ix, charval := range []byte(consumer) {
		req.consumer[ix] = charval
	}

	err := ioctl_gpio_v2_line_request(uintptr(line.chip_fd), &req)
	if err == nil {
		line.fd = req.fd
		line.consumer = string(consumer)
	} else {
		err = fmt.Errorf("line_request ioctl: %w", err)
	}
	return line.fd, err
}

func (line *GPIOLine) setOut() error {
	line.direction = LineOutput
	line.edge = gpio.NoEdge
	line.pull = gpio.PullNoChange
	return line.setLine(getFlags(LineOutput, line.edge, line.pull))
}

func (line *GPIOLine) setLine(flags uint64) error {
	req_fd, err := line.getLine()
	if err != nil {
		return err
	}

	var req gpio_v2_line_config
	req.flags = flags
	return ioctl_gpio_v2_line_config(uintptr(req_fd), &req)
}

// Deprecated: Use PinFunc.Func. Will be removed in v4. Function implements pin.Pin.
func (line *GPIOLine) Function() string {
	return string(line.Func())
}

// Func implements pin.PinFunc.
func (line *GPIOLine) Func() pin.Func {
	if line.direction == LineInput {
		if line.Read() {
			return gpio.IN_HIGH
		}
		return gpio.IN_LOW
	} else if line.direction == LineOutput {
		if line.Read() {
			return gpio.OUT_HIGH
		}
		return gpio.OUT_LOW
	}
	return pin.FuncNone
}

// SupportedFuncs implements pin.PinFunc.
func (line *GPIOLine) SupportedFuncs() []pin.Func {
	return []pin.Func{gpio.IN, gpio.OUT}
}

// SetFunc implements pin.PinFunc.
func (line *GPIOLine) SetFunc(f pin.Func) error {
	switch f {
	case gpio.IN:
		return line.In(gpio.PullNoChange, gpio.NoEdge)
	case gpio.OUT_HIGH:
		return line.Out(gpio.High)
	case gpio.OUT, gpio.OUT_LOW:
		return line.Out(gpio.Low)
	default:
		return errors.New("unsupported function")
	}
}

// A representation of a Linux GPIO Chip. A computer may have
// more than one GPIOChip.
type GPIOChip struct {
	// The name of the device as reported by the kernel.
	name string
	// Path represents the path to the /dev/gpiochip* character
	// device used for ioctl() calls.
	path  string
	label string
	// The number of lines this device supports.
	lineCount int
	// The set of Lines associated with this device.
	lines []*GPIOLine
	// The LineSets opened on this device.
	lineSets []*LineSet
	// The file descriptor to the Path device.
	fd uintptr
	// File associated with the file descriptor.
	file   *os.File
	osfile *os.File
}

func (chip *GPIOChip) Name() string {
	return chip.name
}

func (chip *GPIOChip) Path() string {
	return chip.path
}

func (chip *GPIOChip) Label() string {
	return chip.label
}

func (chip *GPIOChip) LineCount() int {
	return chip.lineCount
}

func (chip *GPIOChip) Lines() []*GPIOLine {
	return chip.lines
}

func (chip *GPIOChip) LineSets() []*LineSet {
	return chip.lineSets
}

// Construct a new GPIOChip by opening the /dev/gpiochip*
// path specified and using Kernel ioctl() calls to
// read information about the chip and it's associated lines.
func newGPIOChip(path string) (*GPIOChip, error) {
	chip := GPIOChip{path: path}
	f, err := os.OpenFile(path, os.O_RDONLY, 0400)
	if err != nil {
		err = fmt.Errorf("opening gpio chip %s failed. error: %w", path, err)
		log.Println(err)
		return nil, err
	}
	chip.file = f
	chip.fd = chip.file.Fd()
	// failure to maintain a reference leads to the file being garbage
	// collected and the handle closed...
	chip.osfile = os.NewFile(uintptr(chip.fd), "GPIO Chip - "+path)
	var info gpiochip_info
	err = ioctl_gpiochip_info(chip.fd, &info)
	if err != nil {
		log.Printf("newGPIOChip: %s\n", err)
		return nil, fmt.Errorf("newgpiochip %s: %w", path, err)
	}

	chip.name = strings.Trim(string(info.name[:]), "\x00")
	chip.label = strings.Trim(string(info.label[:]), "\x00")
	if len(chip.label) == 0 {
		chip.label = chip.name
	}
	chip.lineCount = int(info.lines)
	var line_info gpio_v2_line_info
	for line := 0; line < int(info.lines); line++ {
		line_info.offset = uint32(line)
		err := ioctl_gpio_v2_line_info(chip.fd, &line_info)
		if err != nil {
			log.Println("newGPIOChip get line info", err)
			return nil, fmt.Errorf("reading line info: %w", err)
		}
		line := newGPIOLine(uint32(line), string(line_info.name[:]), string(line_info.consumer[:]), chip.fd)
		chip.lines = append(chip.lines, line)
	}
	return &chip, nil
}

// Close closes the file descriptor associated with the chipset,
// along with any configured Lines and LineSets.
func (chip *GPIOChip) Close() {
	_ = chip.file.Close()
	_ = chip.osfile.Close()
	chip.file = nil
	chip.osfile = nil
	chip.fd = 0
	for _, line := range chip.lines {
		if line.fd != 0 {
			line.Close()
		}
	}
	for _, lineset := range chip.lineSets {
		_ = lineset.Close()
	}
	_ = syscall_close_wrapper(int(chip.fd))
}

// ByName returns a GPIOLine for a specific name. If not
// found, returns nil.
func (chip *GPIOChip) ByName(name string) *GPIOLine {
	for _, line := range chip.lines {
		if line.name == name {
			return line
		}
	}
	return nil
}

// ByNumber returns a line by it's specific GPIO Chip line
// number. Note this has NO RELATIONSHIP to a pin # on
// a board.
func (chip *GPIOChip) ByNumber(number int) *GPIOLine {
	if number < 0 || number >= len(chip.lines) {
		log.Printf("GPIOChip.ByNumber(%d) with out of range value.", number)
		return nil
	}
	return chip.lines[number]
}

// getFlags accepts a set of GPIO configuration values and returns an
// appropriate uint64 ioctl gpio flag.
func getFlags(dir LineDir, edge gpio.Edge, pull gpio.Pull) uint64 {
	var flags uint64
	if dir == LineInput {
		flags |= _GPIO_V2_LINE_FLAG_INPUT
	} else if dir == LineOutput {
		flags |= _GPIO_V2_LINE_FLAG_OUTPUT
	}
	if pull == gpio.PullUp {
		flags |= _GPIO_V2_LINE_FLAG_BIAS_PULL_UP
	} else if pull == gpio.PullDown {
		flags |= _GPIO_V2_LINE_FLAG_BIAS_PULL_DOWN
	}
	if edge == gpio.RisingEdge {
		flags |= _GPIO_V2_LINE_FLAG_EDGE_RISING
	} else if edge == gpio.FallingEdge {
		flags |= _GPIO_V2_LINE_FLAG_EDGE_FALLING
	} else if edge == gpio.BothEdges {
		flags |= _GPIO_V2_LINE_FLAG_EDGE_RISING | _GPIO_V2_LINE_FLAG_EDGE_FALLING
	}
	return flags
}

// Create a LineSet using the configuration specified by config.
func (chip *GPIOChip) LineSetFromConfig(config *LineSetConfig) (*LineSet, error) {
	lines := make([]uint32, len(config.Lines))
	for ix, name := range config.Lines {
		gpioLine := chip.ByName(name)
		if gpioLine == nil {
			return nil, fmt.Errorf("line %s not found in chip %s", name, chip.Name())
		}
		lines[ix] = uint32(gpioLine.Number())
	}
	req := config.getLineSetRequestStruct(lines)

	err := ioctl_gpio_v2_line_request(chip.fd, req)
	if err != nil {
		return nil, fmt.Errorf("LineSetFromConfig: %w", err)
	}
	ls := LineSet{fd: req.fd}

	for offset, lineName := range config.Lines {
		lsl := chip.newLineSetLine(int(chip.ByName(lineName).Number()), offset, config)
		lsl.parent = &ls
		ls.lines = append(ls.lines, lsl)
	}

	return &ls, nil
}

// Create a representation of a specific line in the set.
func (chip *GPIOChip) newLineSetLine(line_number, offset int, config *LineSetConfig) *LineSetLine {
	line := chip.ByNumber(line_number)
	lsl := &LineSetLine{
		number:    uint32(line_number),
		offset:    uint32(offset),
		name:      line.Name(),
		direction: config.DefaultDirection,
		pull:      config.DefaultPull,
		edge:      config.DefaultEdge}

	for _, override := range config.Overrides {
		for _, overrideLine := range override.Lines {
			if overrideLine == line.Name() {
				lsl.direction = override.Direction
				lsl.edge = override.Edge
				lsl.pull = override.Pull

			}
		}
	}
	return lsl
}

func (chip *GPIOChip) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name      string      `json:"Name"`
		Path      string      `json:"Path"`
		Label     string      `json:"Label"`
		LineCount int         `json:"LineCount"`
		Lines     []*GPIOLine `json:"Lines"`
		LineSets  []*LineSet  `json:"LineSets"`
	}{
		Name:      chip.Name(),
		Path:      chip.Path(),
		Label:     chip.Label(),
		LineCount: chip.LineCount(),
		Lines:     chip.lines,
		LineSets:  chip.lineSets})
}

// String returns the chip information, and line information in JSON format.
func (chip *GPIOChip) String() string {
	json, _ := json.MarshalIndent(chip, "", "    ")
	return string(json)
}

// LineSet requests a set of io pins and configures them according to the
// parameters. Using a LineSet, you can perform IO operations on multiple
// lines in a single operation. For more control, see LineSetFromConfig.
func (chip *GPIOChip) LineSet(defaultDirection LineDir, defaultEdge gpio.Edge, defaultPull gpio.Pull, lines ...string) (*LineSet, error) {
	cfg := &LineSetConfig{DefaultDirection: defaultDirection, DefaultEdge: defaultEdge, DefaultPull: defaultPull}
	for _, lineName := range lines {
		p := chip.ByName(lineName)
		if p == nil {
			return nil, fmt.Errorf("line %s not found", lineName)
		}
		cfg.Lines = append(cfg.Lines, p.Name())
	}
	return chip.LineSetFromConfig(cfg)
}

// driverGPIO implements periph.Driver.
type driverGPIO struct {
	_ string
}

func (d *driverGPIO) String() string {
	return "ioctl-gpio"
}

func (d *driverGPIO) Prerequisites() []string {
	return nil
}

func (d *driverGPIO) After() []string {
	return nil
}

// Init initializes GPIO ioctl handling code.
//
// # Uses Linux gpio ioctl as described at
//
// https://docs.kernel.org/userspace-api/gpio/chardev.html
func (d *driverGPIO) Init() (bool, error) {
	if runtime.GOOS != "linux" {
		return true, nil
	}
	items, err := filepath.Glob("/dev/gpiochip*")
	if err != nil {
		return true, fmt.Errorf("gpioioctl: %w", err)
	}
	if len(items) == 0 {
		return false, errors.New("no GPIO chips found")
	}
	// First, get all of the chips on the system.
	var chips []*GPIOChip
	var chip *GPIOChip
	for _, item := range items {
		chip, err = newGPIOChip(item)
		if err == nil {
			chips = append(chips, chip)
		} else {
			log.Println("gpioioctl.driverGPIO.Init() Error", err)
		}
	}
	// Now, sort the chips so that those labeled with pinctrl- ( a Pi kernel standard)
	// come first. Otherwise, sort them by label. This _should_ protect us from any
	// random changes in chip naming/ordering.
	sort.Slice(chips, func(i, j int) bool {
		I := chips[i]
		J := chips[j]
		if strings.HasPrefix(I.Label(), "pinctrl-") {
			if strings.HasPrefix(J.Label(), "pinctrl-") {
				return I.Label() < J.Label()
			}
			return true
		} else if strings.HasPrefix(J.Label(), "pinctrl-") {
			return false
		}
		return I.Label() < J.Label()
	})

	mName := make(map[string]struct{})
	// Get a list of already registered GPIO Line names.
	registeredPins := make(map[string]struct{})
	for _, pin := range gpioreg.All() {
		registeredPins[pin.Name()] = struct{}{}
	}

	// Now, iterate over the chips we found and add their lines to conn/gpio/gpioreg
	for _, chip := range chips {
		// On a pi, gpiochip0 is also symlinked to gpiochip4, checking the map
		// ensures we don't duplicate the chip.
		if _, found := mName[chip.Name()]; found {
			chip.Close()
		} else {
			Chips = append(Chips, chip)
			mName[chip.Name()] = struct{}{}
			// Now, iterate over the lines on this chip.
			for _, line := range chip.lines {
				// If the line has some sort of reasonable name...
				if len(line.name) > 0 && line.name != "_" && line.name != "-" {
					// See if the name is already registered. On the Pi5, there are at
					// least two chips that export "2712_WAKE" as the line name.
					if _, ok := registeredPins[line.Name()]; ok {
						// This is a duplicate name. Prefix the line name with the
						// chip name.
						line.name = chip.Name() + "-" + line.Name()
						if _, found := registeredPins[line.Name()]; found {
							// It's still not unique. Skip it.
							continue
						}
					}
					registeredPins[line.Name()] = struct{}{}
					if err = gpioreg.Register(line); err != nil {
						log.Println("chip", chip.Name(), " gpioreg.Register(line) ", line, " returned ", err)
					}
				}
			}
		}
	}
	return len(Chips) > 0, nil
}

var drvGPIO driverGPIO

func init() {
	// Init our consumer name. It's used when a line is requested, and
	// allows utility programs like gpioinfo to find out who has a line
	// open.
	fname := path.Base(os.Args[0])
	s := fmt.Sprintf("%s@%d", fname, os.Getpid())
	charBytes := []byte(s)
	if len(charBytes) >= _GPIO_MAX_NAME_SIZE {
		charBytes = charBytes[:_GPIO_MAX_NAME_SIZE-1]
	}
	consumer = charBytes

	driverreg.MustRegister(&drvGPIO)
}

// Ensure that Interfaces for these types are implemented fully.
var _ gpio.PinIO = &GPIOLine{}
var _ gpio.PinIn = &GPIOLine{}
var _ gpio.PinOut = &GPIOLine{}
var _ pin.PinFunc = &GPIOLine{}
