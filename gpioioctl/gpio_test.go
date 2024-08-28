package gpioioctl

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
//
// The In/Out tests depend upon having a jumper wire connecting _OUT_LINE and
// _IN_LINE

import (
	"testing"
	"time"

	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

const (
	_OUT_LINE = "GPIO5"
	_IN_LINE  = "GPIO13"
)

func init() {
	_, _ = driverreg.Init()
}

func TestChips(t *testing.T) {
	if len(Chips) <= 0 {
		t.Fatalf("Chips contains no entries.")
	}
	chip := Chips[0]
	if len(chip.Name) == 0 {
		t.Errorf("chip.Name() is 0 length")
	}
	if len(chip.Lines) != chip.LineCount {
		t.Errorf("Incorrect line count. Found: %d for LineCount, Returned Lines length=%d", chip.LineCount, len(chip.Lines))
	}
	s := chip.String()
	if len(s) == 0 {
		t.Error("Error calling chip.String(). No output returned!")
	}
}

func TestGPIORegistryByName(t *testing.T) {
	outLine := gpioreg.ByName(_OUT_LINE)
	if outLine == nil {
		t.Fatalf("Error retrieving GPIO Line %s", _OUT_LINE)
	}
	if outLine.Name() != _OUT_LINE {
		t.Errorf("Error checking name. Expected %s, received %s", _OUT_LINE, outLine.Name())
	}

	if outLine.Number() < 0 || outLine.Number() >= len(Chips[0].Lines) {
		t.Errorf("Invalid chip number %d received for %s", outLine.Number(), _OUT_LINE)
	}
}

func TestConsumer(t *testing.T) {
	chip := Chips[0]
	l := chip.ByName(_OUT_LINE)
	if l == nil {
		t.Fatalf("Error retrieving GPIO Line %s", _OUT_LINE)
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
	l := chip.ByName(_OUT_LINE)
	if l == nil {
		t.Fatalf("Error retrieving GPIO Line %s", _OUT_LINE)
	}
	if l.Number() < 0 || l.Number() >= chip.LineCount {
		t.Errorf("line.Number() returned value (%d) out of range", l.Number())
	}
	l2 := chip.ByNumber(l.Number())
	if l2 == nil {
		t.Errorf("retrieve Line from chip by number %d failed.", l.Number())
	}

}

func TestString(t *testing.T) {
	line := gpioreg.ByName(_OUT_LINE)
	if line == nil {
		t.Fatalf("Error retrieving GPIO Line %s", _OUT_LINE)
	}
	s := line.String()
	if len(s) == 0 {
		t.Errorf("GPIOLine.String() failed.")
	}
}

func TestWriteReadSinglePin(t *testing.T) {
	var err error
	chip := Chips[0]
	inLine := chip.ByName(_IN_LINE)
	outLine := chip.ByName(_OUT_LINE)
	defer inLine.Close()
	defer outLine.Close()
	err = outLine.Out(true)
	if err != nil {
		t.Errorf("outLine.Out() %s", err)
	}
	if val := inLine.Read(); !val {
		t.Error("Error reading/writing GPIO Pin. Expected true, received false!")
	}
	if inLine.Pull()!=gpio.PullUp {
		t.Errorf("Pull() returned %s expected %s",pullLabels[inLine.Pull()],pullLabels[gpio.PullUp])
	}
	err = outLine.Out(false)
	if err != nil {
		t.Errorf("outLine.Out() %s", err)
	}
	if val := inLine.Read(); val {
		t.Error("Error reading/writing GPIO Pin. Expected false, received true!")
	}
	/*
		By Design, lines should auto change directions if Read()/Out() are called
		and they don't match.
	*/
	err = inLine.Out(false)
	if err != nil {
		t.Errorf("inLine.Out() %s", err)
	}
	time.Sleep(500 * time.Millisecond)
	err = inLine.Out(true)
	if err != nil {
		t.Errorf("inLine.Out() %s", err)
	}
	if val := outLine.Read(); !val {
		t.Error("Error read/writing with auto-reverse of line functions.")
	}
	err = inLine.Out(false)
	if err != nil {
		t.Errorf("TestWriteReadSinglePin() %s", err)
	}
	if val := outLine.Read(); val {
		t.Error("Error read/writing with auto-reverse of line functions.")
	}

}

func clearEdges(line gpio.PinIn) bool {
	result := false
	for line.WaitForEdge(10 * time.Millisecond) {
		result = true
	}
	return result
}

func TestWaitForEdgeTimeout(t *testing.T) {
	line := Chips[0].ByName(_IN_LINE)
	defer line.Close()
	err := line.In(gpio.PullUp, gpio.BothEdges)
	if err != nil {
		t.Error(err)
	}
	clearEdges(line)
	tStart := time.Now().UnixMilli()
	line.WaitForEdge(5 * time.Second)
	tEnd := time.Now().UnixMilli()
	tDiff := tEnd - tStart
	if tDiff < 4500 || tDiff > 5500 {
		t.Errorf("timeout duration failure. Expected duration: 5000, Actual duration: %d", tDiff)
	}
}

// Test detection of rising, falling, and both.
func TestWaitForEdgeSinglePin(t *testing.T) {
	tests := []struct {
		startVal gpio.Level
		edge     gpio.Edge
		writeVal gpio.Level
	}{
		{startVal: false, edge: gpio.RisingEdge, writeVal: true},
		{startVal: true, edge: gpio.FallingEdge, writeVal: false},
		{startVal: false, edge: gpio.BothEdges, writeVal: true},
		{startVal: true, edge: gpio.BothEdges, writeVal: false},
	}
	var err error
	line := Chips[0].ByName(_IN_LINE)
	outLine := Chips[0].ByName(_OUT_LINE)
	defer line.Close()
	defer outLine.Close()

	for _, test := range tests {
		err = outLine.Out(test.startVal)
		if err != nil {
			t.Errorf("set initial value. %s", err)
		}
		err = line.In(gpio.PullUp, test.edge)
		if err != nil {
			t.Errorf("line.In() %s", err)
		}
		clearEdges(line)
		err = outLine.Out(test.writeVal)
		if err != nil {
			t.Errorf("outLine.Out() %s", err)
		}
		if edgeReceived := line.WaitForEdge(time.Second); !edgeReceived {
			t.Errorf("Expected Edge %s was not received on transition from %t to %t", edgeLabels[test.edge], test.startVal, test.writeVal)
		}
	}
}

func TestHalt(t *testing.T) {
	line := Chips[0].ByName(_IN_LINE)
	defer line.Close()
	err := line.In(gpio.PullUp, gpio.BothEdges)
	if err != nil {
		t.Fatalf("TestHalt() %s", err)
	}
	clearEdges(line)
	// So what we'll do here is setup a goroutine to wait three seconds and then send a halt.
	go func() {
		time.Sleep(time.Second * 3)
		err = line.Halt()
		if err != nil {
			t.Error(err)
		}
	}()
	tStart := time.Now().UnixMilli()
	line.WaitForEdge(time.Second * 30)
	tEnd := time.Now().UnixMilli()
	tDiff := tEnd - tStart
	if tDiff > 3500 {
		t.Errorf("error calling halt to interrupt WaitForEdge() Duration %d exceeded expected value.",tDiff)
	}
}
