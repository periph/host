package gpioioctl

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
//
// Basic tests. More complete test is contained in the
// periph.io/x/cmd/v3/periph-smoketest/gpiosmoketest
// folder.
import (
	"testing"

	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

var _test_line *GPIOLine

func init() {
	_, _ = driverreg.Init()
}

func TestChips(t *testing.T) {
	if len(Chips) <= 0 {
		t.Fatal("Chips contains no entries.")
	}
	chip := Chips[0]
	if len(chip.Name()) == 0 {
		t.Error("chip.Name() is 0 length")
	}
	if len(chip.Path())==0 {
		t.Error("chip path is 0 length")
	}
	if len(chip.Label())==0 {
		t.Error("chip label is 0 length!")
	}
	if len(chip.Lines()) != chip.LineCount() {
		t.Errorf("Incorrect line count. Found: %d for LineCount, Returned Lines length=%d", chip.LineCount(), len(chip.Lines()))
	}
	for _, line := range chip.Lines() {
		if len(line.Consumer()) == 0 {
			_test_line = line
			break
		}
	}
	if _test_line == nil {
		t.Error("Error finding unused line for testing!")
	}
	for _, c := range Chips {
		s := c.String()
		if len(s) == 0 {
			t.Error("Error calling chip.String(). No output returned!")
		} else {
			t.Log(s)
		}

	}
	
}

func TestGPIORegistryByName(t *testing.T) {
	outLine := gpioreg.ByName(_test_line.Name())
	if outLine == nil {
		t.Fatalf("Error retrieving GPIO Line %s", _test_line.Name())
	}
	if outLine.Name() != _test_line.Name() {
		t.Errorf("Error checking name. Expected %s, received %s", _test_line.Name(), outLine.Name())
	}

	if outLine.Number() < 0 || outLine.Number() >= len(Chips[0].Lines()) {
		t.Errorf("Invalid chip number %d received for %s", outLine.Number(), _test_line.Name())
	}
}

// Test the consumer field. Since this actually configures a line for output,
// it actually tests a fair amount of the code to request a line, and configure
// it.
func TestConsumer(t *testing.T) {

	l := Chips[0].ByName(_test_line.Name())
	if l == nil {
		t.Fatalf("Error retrieving GPIO Line %s", _test_line.Name())
	}
	defer l.Close()
	// Consumer isn't written until the line is configured.
	err := l.Out(true)
	if err != nil {
		t.Errorf("l.Out() %s", err)
	}
	if l.Consumer() != string(consumer) {
		t.Errorf("Incorrect consumer name. Expected consumer name %s on line. received empty %s", string(consumer), l.Consumer())
	}
}

func TestNumber(t *testing.T) {
	chip := Chips[0]
	l := chip.ByName(_test_line.Name())
	if l == nil {
		t.Fatalf("Error retrieving GPIO Line %s", _test_line.Name())
	}
	if l.Number() < 0 || l.Number() >= chip.LineCount() {
		t.Errorf("line.Number() returned value (%d) out of range", l.Number())
	}
	l2 := chip.ByNumber(l.Number())
	if l2 == nil {
		t.Errorf("retrieve Line from chip by number %d failed.", l.Number())
	}

}

func TestString(t *testing.T) {
	line := gpioreg.ByName(_test_line.Name())
	if line == nil {
		t.Fatalf("Error retrieving GPIO Line %s", _test_line.Name())
	}
	s := line.String()
	if len(s) == 0 {
		t.Errorf("GPIOLine.String() failed.")
	}
}
