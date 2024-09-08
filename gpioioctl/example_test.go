package gpioioctl_test

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

import (
	"fmt"
	"log"
	"time"

	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/gpioioctl"
)

func Example() {
	_, _ = host.Init()
	_, _ = driverreg.Init()

	fmt.Println("GPIO Test Program")
	chip := gpioioctl.Chips[0]
	defer chip.Close()
	fmt.Println(chip.String())
	// Test by flashing an LED.
	led := gpioreg.ByName("GPIO5")
	fmt.Println("Flashing LED ", led.Name())
	for i := range 20 {
		_ = led.Out((i % 2) == 0)
		time.Sleep(500 * time.Millisecond)
	}
	_ = led.Out(true)

	testRotary(chip, "GPIO20", "GPIO21", "GPIO19")
}

// Test the LineSet functionality by using it to read a Rotary Encoder w/ Button.
func testRotary(chip *gpioioctl.GPIOChip, stateLine, dataLine, buttonLine string) {
	config := gpioioctl.LineSetConfig{DefaultDirection: gpioioctl.LineInput, DefaultEdge: gpio.RisingEdge, DefaultPull: gpio.PullUp}
	config.Lines = []string{stateLine, dataLine, buttonLine}
	// The Data Pin of the Rotary Encoder should NOT have an edge.
	_ = config.AddOverrides(gpioioctl.LineInput, gpio.NoEdge, gpio.PullUp, dataLine)
	ls, err := chip.LineSetFromConfig(&config)
	if err != nil {
		log.Fatal(err)
	}
	defer ls.Close()
	statePinNumber := uint32(ls.ByOffset(0).Number())
	buttonPinNumber := uint32(ls.ByOffset(2).Number())

	var tLast = time.Now().Add(-1 * time.Second)
	var halting bool
	go func() {
		time.Sleep(60 * time.Second)
		halting = true
		fmt.Println("Sending halt!")
		_ = ls.Halt()
	}()
	fmt.Println("Test Rotary Switch - Turn dial to test rotary encoder, press button to test it.")
	for {
		lineNumber, _, err := ls.WaitForEdge(0)
		if err == nil {
			tNow := time.Now()
			if (tNow.UnixMilli() - tLast.UnixMilli()) < 100 {
				continue
			}
			tLast = tNow
			if lineNumber == statePinNumber {
				var bits uint64
				tDeadline := tNow.UnixNano() + 20_000_000
				var consecutive uint64
				for time.Now().UnixNano() < tDeadline {
					// Spin on reading the pins until we get some number
					// of consecutive readings that are the same.
					bits, _ = ls.Read(0x03)
					if bits&0x01 == 0x00 {
						// We're bouncing.
						consecutive = 0
					} else {
						consecutive += 1
						if consecutive > 25 {
							if bits == 0x01 {
								fmt.Printf("Clockwise bits=%d\n", bits)
							} else if bits == 0x03 {
								fmt.Printf("CounterClockwise bits=%d\n", bits)
							}
							break
						}
					}
				}
			} else if lineNumber == buttonPinNumber {
				fmt.Println("Button Pressed!")
			}
		} else {
			fmt.Println("Timeout detected")
			if halting {
				break
			}
		}
	}
}
