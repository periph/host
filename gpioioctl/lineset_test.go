package gpioioctl

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
//
// This is the set of tests for the LineSet functionality.

import (
	"testing"
	"time"

	"periph.io/x/conn/v3/gpio"
)

var outputLines = []string{"GPIO2", "GPIO3", "GPIO4", "GPIO5", "GPIO6", "GPIO7", "GPIO8", "GPIO9"}
var inputLines = []string{"GPIO10", "GPIO11", "GPIO12", "GPIO13", "GPIO14", "GPIO15", "GPIO16", "GPIO17"}

func verifyLineSet(t *testing.T,
	chip *GPIOChip,
	lsTest *LineSet,
	direction LineDir,
	lines []string) {
	lsTestLines := lsTest.Lines()
	if lsTest.LineCount() != len(lines) || len(lsTestLines) != lsTest.LineCount() {
		t.Errorf("lineset does not match length of lines %#v", lines)
	}
	s := lsTest.String()
	if len(s) == 0 {
		t.Error("error - empty string")
	}

	// Verify each individual line is as expected.
	for ix, lineName := range lines {
		lsl := lsTestLines[ix]
		line := chip.ByName(lineName)
		if lsl.Offset() != uint32(ix) {
			t.Errorf("Unexpected offset in LineSetLine. Expected: %d Found %d", ix, lsl.Offset())
		}
		if lsl.Name() != line.Name() {
			t.Errorf("Expected LineSetLine.Name()=%s, found %s", lineName, lsl.Name())
		}
		if lsl.Number() != line.Number() {
			t.Errorf("Expected LineSetLine.Number()=%d, found %d", line.Number(), lsl.Number())
		}
		if lsl.Offset() != uint32(ix) {
			t.Errorf("Line # %d, expected offset %d, got %d", lsl.Number(), ix, lsl.Offset())
		}
		if lsl.direction != direction {
			t.Errorf("Expected LineSetLine.direction=%s, found %s", directionLabels[direction], directionLabels[lsl.direction])
		}
		s := lsl.String()
		if len(s) == 0 {
			t.Error("LineSetLine.String() returned empty string.")
		}
		e := lsl.Halt()
		if e == nil {
			t.Error("LineSetLine.Halt() should return an error!")
		}
	}
}

func createLineSets(t *testing.T, chip *GPIOChip, edge gpio.Edge) (lsOutput *LineSet, lsInput *LineSet) {
	// Create the Output Lineset
	lsOutput, err := chip.LineSet(LineOutput, gpio.NoEdge, gpio.PullNoChange, outputLines...)
	if err != nil {
		t.Fatalf("Error creating output LineSet %s", err.Error())
	}

	verifyLineSet(t, chip, lsOutput, LineOutput, outputLines)

	// Create the Input LineSet
	lsInput, err = chip.LineSet(LineInput, edge, gpio.PullUp, inputLines...)
	if err != nil {
		t.Fatalf("Error creating input LineSet %s", err.Error())
	}

	verifyLineSet(t, chip, lsInput, LineInput, inputLines)
	return
}

// Test Creating the line set and verify the pin setups.
func TestLineSetCreation(t *testing.T) {
	chip := Chips[0]
	lsOutput, lsInput := createLineSets(t, chip, gpio.NoEdge)
	if lsOutput != nil {
		errClose := lsOutput.Close()
		if errClose != nil {
			t.Errorf("Closing Output LineSet %v", errClose)
		}
	}
	if lsInput != nil {
		errClose := lsInput.Close()
		if errClose != nil {
			t.Errorf("Closing Output LineSet %v", errClose)
		}

	}
}

// Test writing to the output set and reading from the input set.
func TestLineSetReadWrite(t *testing.T) {
	chip := Chips[0]
	lsOutput, lsInput := createLineSets(t, chip, gpio.NoEdge)
	if lsOutput == nil || lsInput == nil {
		return
	}
	defer lsOutput.Close()
	defer lsInput.Close()
	limit := (1 << len(outputLines)) - 1
	mask := uint64(limit)
	for i := range limit {
		// Test Read of all pins in the set at once.
		//
		// Generally, if this is failing double-check your
		// jumper wires between pins.
		//
		err := lsOutput.Out(uint64(i), mask)
		if err != nil {
			t.Errorf("Error writing to output set. Error=%s", err.Error())
			break
		}
		val, err := lsInput.Read(0)
		if err != nil {
			t.Error(err)
		}
		if val != uint64(i) {
			t.Errorf("Error on input. Expected %d, Received: %d", i, val)
		}
		// Now, test the value obtained by reading each pin in the set
		// individually.
		var sum uint64
		for ix, line := range lsInput.Lines() {
			if lineVal := line.Read(); lineVal {
				sum += uint64(1 << ix)
			}
		}
		if sum != uint64(i) {
			t.Errorf("Error reading pins individually and summing them. Expected value: %d, Summed Value: %d", i, sum)
		}
	}
}

func clearLineSetEdges(ls *LineSet) bool {
	result := false
	for {
		_, _, err := ls.WaitForEdge(10 * time.Millisecond)
		if err == nil {
			result = true
		} else {
			// It timed out, so it's empty.
			break
		}
	}
	return result
}

// Test the timeout function of the LineSet WaitForEdge
func TestLineSetWaitForEdgeTimeout(t *testing.T) {
	lsOutput, lsInput := createLineSets(t, Chips[0], gpio.RisingEdge)
	lsOutput.Close()
	defer lsInput.Close()
	clearLineSetEdges(lsInput)
	tStart := time.Now().UnixMilli()
	_, _, _ = lsInput.WaitForEdge(5 * time.Second)
	tEnd := time.Now().UnixMilli()
	tDiff := tEnd - tStart
	if tDiff < 4500 || tDiff > 5500 {
		t.Errorf("timeout duration failure. Expected duration: 5000, Actual duration: %d", tDiff)
	}
}

// Test the halt function successfully interupts a WaitForEdge()
func TestLineSetHalt(t *testing.T) {
	chip := Chips[0]
	lsOutput, lsInput := createLineSets(t, chip, gpio.BothEdges)
	if lsOutput == nil || lsInput == nil {
		return
	}
	lsOutput.Close() // Don't need it.
	defer lsInput.Close()

	clearLineSetEdges(lsInput)
	// So what we'll do here is setup a goroutine to wait three seconds and then send a halt.
	go func() {
		time.Sleep(time.Second * 3)
		err := lsInput.Halt()
		if err != nil {
			t.Error(err)
		}
	}()
	tStart := time.Now().UnixMilli()
	_, _, _ = lsInput.WaitForEdge(time.Second * 30)
	tEnd := time.Now().UnixMilli()
	tDiff := tEnd - tStart
	if tDiff > 3500 {
		t.Errorf("error calling halt to interrupt LineSet.WaitForEdge() Duration not as expected. Actual Duration: %d",tDiff)
	}
}

// Execute WaitForEdge tests. The implementation ensures that the
// LineSetLine.Out() and LineSetLine.Read() functions work as
// expected too.
func TestLineSetWaitForEdge(t *testing.T) {
	// Step 1 - Get the LineSets
	chip := Chips[0]
	lsOutput, err := chip.LineSet(LineOutput, gpio.NoEdge, gpio.PullNoChange, outputLines...)
	if lsOutput == nil {
		t.Errorf("Error creating output lineset. %s", err)
	}
	defer lsOutput.Close()
	tests := []struct {
		initValue    gpio.Level
		edgeSet      gpio.Edge
		expectedEdge gpio.Edge
		writeValue   gpio.Level
	}{
		{initValue: false, edgeSet: gpio.RisingEdge, expectedEdge: gpio.RisingEdge, writeValue: true},
		{initValue: true, edgeSet: gpio.FallingEdge, expectedEdge: gpio.FallingEdge, writeValue: false},
		{initValue: false, edgeSet: gpio.BothEdges, expectedEdge: gpio.RisingEdge, writeValue: true},
		{initValue: true, edgeSet: gpio.BothEdges, expectedEdge: gpio.FallingEdge, writeValue: false},
	}
	for _, test := range tests {
		lsInput, err := chip.LineSet(LineInput, test.edgeSet, gpio.PullUp, inputLines...)
		if err != nil {
			t.Error(err)
			return
		}
		for ix, line := range lsOutput.Lines() {
			inLine := lsInput.Lines()[ix]
			// Write the initial value.
			err = line.Out(test.initValue)
			if err != nil {
				t.Error(err)
				continue
			}
			// Clear any queued events.
			clearLineSetEdges(lsInput)
			go func() {
				// Write that line to high
				err := line.Out(test.writeValue)
				if err != nil {
					t.Error(err)
				}
			}()
			// lineTriggered is the line number.
			lineTriggered, edge, err := lsInput.WaitForEdge(time.Second)
			if err == nil {
				if lineTriggered != uint32(inLine.Number()) {
					t.Errorf("Test: %#v expected line: %d triggered line: %d", test, lineTriggered, line.Number())
				}
				if edge != test.expectedEdge {
					t.Errorf("Test: %#v expected edge: %s received edge: %s", test, edgeLabels[test.expectedEdge], edgeLabels[edge])
				}

				if inLine.Read() != test.writeValue {
					t.Errorf("Test: %#v received %t expected %t", test, inLine.Read(), test.writeValue)
				}
			} else {
				t.Errorf("Test: %#v Line Offset: %d Error: %s", test, ix, err)
			}

		}
		lsInput.Close()
	}
}

// Test LineSetFromConfig with an Override on one line.
func TestLineSetConfigWithOverride(t *testing.T) {
	chip := Chips[0]
	line0 := chip.ByName(outputLines[0])
	line1 := chip.ByName(outputLines[1])
	cfg := LineSetConfig{
		Lines:            []string{line0.Name(), line1.Name()},
		DefaultDirection: LineOutput,
		DefaultEdge:      gpio.NoEdge,
		DefaultPull:      gpio.PullNoChange,
	}
	err := cfg.AddOverrides(LineInput, gpio.RisingEdge, gpio.PullUp, []string{line1.Name()}...)
	if err != nil {
		t.Errorf("AddOverrides() %s", err)
	}
	ls, err := chip.LineSetFromConfig(&cfg)
	if err != nil {
		t.Errorf("Error creating lineset with override. %s", err)
		return
	}
	if ls == nil {
		t.Error("Error creating lineset. Returned value=nil")
		return
	}
	defer ls.Close()
	lsl := ls.ByNumber(line0.Number())
	if lsl.Number() != line0.Number() {
		t.Errorf("LineSetLine pin 0 not as expected. Number=%d Expected: %d", lsl.Number(), line0.Number())
	}
	if lsl.direction != LineOutput {
		t.Error("LineSetLine override direction!=LineOutput")
	}
	if lsl.edge != gpio.NoEdge {
		t.Error("LineSetLine override, edge!=gpio.NoEdge")
	}
	if lsl.Pull() != gpio.PullNoChange {
		t.Error("LineSetLine override pull!=gpio.PullUp")
	}

	lsl = ls.ByNumber(line1.Number())
	if lsl.direction != LineInput {
		t.Errorf("LineSetLine override direction!=LineInput ls=%s", lsl)
	}
	if lsl.edge != gpio.RisingEdge {
		t.Error("LineSetLine override, edge!=gpio.RisingEdge")
	}
	if lsl.Pull() != gpio.PullUp {
		t.Error("LineSetLine override pull!=gpio.PullUp")
	}
}
