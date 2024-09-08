package gpioioctl

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Basic tests. More complete test is contained in the
// periph.io/x/cmd/v3/periph-smoketest/gpiosmoketest
// folder.
import (
	"log"
	"testing"

	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

var testLine *GPIOLine

func init() {
	_, err := driverreg.Init()
	if err != nil {
		log.Println(err)
	}

	if len(Chips) == 0 {
		/*
		   During pipeline builds, GPIOChips may not be available, or
		   it may build on another OS. In that case, mock in enough
		   for a test to pass.
		*/
		line := GPIOLine{
			number:    0,
			name:      "DummyGPIOLine",
			consumer:  "",
			edge:      gpio.NoEdge,
			pull:      gpio.PullNoChange,
			direction: LineDirNotSet,
		}

		chip := GPIOChip{name: "DummyGPIOChip",
			path:      "/dev/gpiochipdummy",
			label:     "Dummy GPIOChip for Testing Purposes",
			lineCount: 1,
			lines:     []*GPIOLine{&line},
		}
		Chips = append(Chips, &chip)
		if err = gpioreg.Register(&line); err != nil {
			log.Println("chip", chip.Name(), " gpioreg.Register(line) ", line, " returned ", err)
		}
	}
}

func TestChips(t *testing.T) {
	chip := Chips[0]
	if len(chip.Name()) == 0 {
		t.Error("chip.Name() is 0 length")
	}
	if len(chip.Path()) == 0 {
		t.Error("chip path is 0 length")
	}
	if len(chip.Label()) == 0 {
		t.Error("chip label is 0 length!")
	}
	if len(chip.Lines()) != chip.LineCount() {
		t.Errorf("Incorrect line count. Found: %d for LineCount, Returned Lines length=%d", chip.LineCount(), len(chip.Lines()))
	}
	for _, line := range chip.Lines() {
		if len(line.Consumer()) == 0 {
			testLine = line
			break
		}
	}
	if testLine == nil {
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
	outLine := gpioreg.ByName(testLine.Name())
	if outLine == nil {
		t.Fatalf("Error retrieving GPIO Line %s", testLine.Name())
	}
	if outLine.Name() != testLine.Name() {
		t.Errorf("Error checking name. Expected %s, received %s", testLine.Name(), outLine.Name())
	}

	if outLine.Number() < 0 || outLine.Number() >= len(Chips[0].Lines()) {
		t.Errorf("Invalid chip number %d received for %s", outLine.Number(), testLine.Name())
	}
}

func TestNumber(t *testing.T) {
	chip := Chips[0]
	l := chip.ByName(testLine.Name())
	if l == nil {
		t.Fatalf("Error retrieving GPIO Line %s", testLine.Name())
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
	line := gpioreg.ByName(testLine.Name())
	if line == nil {
		t.Fatalf("Error retrieving GPIO Line %s", testLine.Name())
	}
	s := line.String()
	if len(s) == 0 {
		t.Errorf("GPIOLine.String() failed.")
	}
}

func TestEscapeJSONString(t *testing.T) {
	testVals := [][]string{
		{"abc def", "abc def"},
		{"abc\"def", "abc\\\"def"},
		{"abc\n\ndef", "abc\\u000A\\u000Adef"},
		{"abc\\def", "abc\\\\def"},
	}
	for _, test := range testVals {
		s := escapeJSONString(test[0])
		if s != test[1] {
			t.Errorf("Error escaping %s, received %s", test[0], s)
		}

	}
}
