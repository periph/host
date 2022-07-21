// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package ftdismoketest is leveraged by periph-smoketest to verify that a
// FT232H/FT232R is working as expectd.
package ftdismoketest

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"adev73/x/host/v3/ftdi"

	"periph.io/x/conn/v3/gpio"
)

// SmokeTest is imported by periph-smoketest.
type SmokeTest struct {
}

// Name implements the SmokeTest interface.
func (s *SmokeTest) Name() string {
	return "ftdi"
}

// Description implements the SmokeTest interface.
func (s *SmokeTest) Description() string {
	return "Tests FT232H/FT232R"
}

// Run implements the SmokeTest interface.
func (s *SmokeTest) Run(f *flag.FlagSet, args []string) (err error) {
	devType := f.String("type", "", "Device type to test, i.e. ft232h or ft232r")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		f.Usage()
		return errors.New("unrecognized arguments")
	}

	all := ftdi.All()
	if len(all) != 1 {
		return fmt.Errorf("exactly one device is expected, got %d", len(all))
	}
	dev := all[0]
	switch *devType {
	case "ft232h":
		if d, ok := dev.(*ftdi.FT232H); ok {
			return testFT232H(d)
		}
	case "ft232r":
		if d, ok := dev.(*ftdi.FT232R); ok {
			return testFT232R(d)
		}
	case "":
		return errors.New("-type is required")
	default:
		return errors.New("unrecognized -type, only ft232h and ft232r are supported")
	}
	return fmt.Errorf("expected %s, got %T; %s", *devType, dev, dev.String())
}

func testFT232H(d *ftdi.FT232H) error {
	// TODO(maruel): Read EEPROM, UA.
	if err := gpioTest(&loggingPin{d.D2}, &loggingPin{d.D1}, false); err != nil {
		return err
	}
	if err := gpioPerfTest(d.C7); err != nil {
		return err
	}
	if err := i2cTest(d); err != nil {
		return err
	}
	return spiTest(d)
}

func testFT232R(d *ftdi.FT232R) error {
	// TODO(maruel): Read EEPROM, UA.
	// TODO(maruel): Remove broken once FT232R driver is stable for GPIO I/O.
	if err := gpioTest(&loggingPin{d.RX}, &loggingPin{d.TX}, true); err != nil {
		return err
	}
	return gpioPerfTest(d.CTS)
}

// gpioPerfTest reads and write in a tight loop to evaluate performance.
//
// It doesn't evaluate correctness.
//
// This makes sure that the flush operation is used, vs relying on
// SetLatencyTimer value.
func gpioPerfTest(p gpio.PinIO) error {
	fmt.Printf("  GPIO performance on %s:\n", p)
	const loops = 1000
	fmt.Printf("    %d reads:  ", loops)
	if err := p.In(gpio.PullNoChange, gpio.NoEdge); err != nil {
		return err
	}
	start := time.Now()
	for i := 0; i < loops; i++ {
		p.Read()
	}
	s := time.Since(start)
	fmt.Printf("%s; %s/op\n", s, s/loops)
	fmt.Printf("    %d writes: ", loops)
	if err := p.Out(gpio.Low); err != nil {
		return err
	}
	start = time.Now()
	for i := 0; i < loops; i++ {
		if err := p.Out(gpio.Low); err != nil {
			return err
		}
	}
	s = time.Since(start)
	fmt.Printf("%s; %s/op\n", s, s/loops)
	return nil
}

// gpioTest ensures connectivity works.
func gpioTest(p1, p2 gpio.PinIO, broken bool) error {
	fmt.Printf("  GPIO functionality on %s and %s:\n", p1, p2)
	if err := p1.In(gpio.PullNoChange, gpio.NoEdge); err != nil {
		return err
	}
	if err := p2.Out(gpio.Low); err != nil {
		return err
	}
	// There can be a small amount of skew. This should inject just enough time.
	time.Sleep(10 * time.Microsecond)
	if l := p1.Read(); l != gpio.Low {
		if broken {
			fmt.Printf("TODO(maruel): Not working; %s: expected to read %s but got %s\n", p1, gpio.Low, l)
		} else {
			return fmt.Errorf("%s: expected to read %s but got %s", p1, gpio.Low, l)
		}
	}
	if err := p2.Out(gpio.High); err != nil {
		return err
	}
	// There can be a small amount of skew. This should inject just enough time.
	time.Sleep(10 * time.Microsecond)
	if l := p1.Read(); l != gpio.High {
		if broken {
			fmt.Printf("TODO(maruel): Not working; %s: expected to read %s but got %s\n", p1, gpio.High, l)
		} else {
			return fmt.Errorf("%s: expected to read %s but got %s", p1, gpio.High, l)
		}
	}
	return nil
}

func i2cTest(d *ftdi.FT232H) error {
	fmt.Printf("  I²C functionality:\n")
	i, err := d.I2C(gpio.Float)
	if err != nil {
		return err
	}
	if err = i.Close(); err != nil {
		return err
	}
	// TODO(maruel): Do a write; this would require a device.
	fmt.Printf("    OK\n")
	return nil
}

func spiTest(d *ftdi.FT232H) error {
	fmt.Printf("  SPI functionality:\n")
	s, err := d.SPI()
	if err != nil {
		return err
	}
	if err = s.Close(); err != nil {
		return err
	}
	// TODO(maruel): Do a write. This can be done without a device.
	fmt.Printf("    OK\n")
	return nil
}

// loggingPin logs when its state changes.
type loggingPin struct {
	gpio.PinIO
}

func (p *loggingPin) In(pull gpio.Pull, edge gpio.Edge) error {
	start := time.Now()
	if err := p.PinIO.In(pull, edge); err != nil {
		fmt.Printf("    %s %s.In(%s, %s) = %v\n", time.Since(start), p, pull, edge, err)
		return err
	}
	fmt.Printf("    %s %s.In(%s, %s)\n", time.Since(start), p, pull, edge)
	return nil
}

func (p *loggingPin) Read() gpio.Level {
	start := time.Now()
	l := p.PinIO.Read()
	fmt.Printf("    %s %s.Read() = %s\n", time.Since(start), p, l)
	return l
}

func (p *loggingPin) Out(l gpio.Level) error {
	start := time.Now()
	if err := p.PinIO.Out(l); err != nil {
		fmt.Printf("    %s %s.Out(%s) = %v\n", time.Since(start), p, l, err)
		return err
	}
	fmt.Printf("    %s %s.Out(%s)\n", time.Since(start), p, l)
	return nil
}
