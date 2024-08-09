// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// This file contains pin mapping information that is specific to the Allwinner
// H6 model.

package allwinner

import (
	_ "embed"
	"encoding/json"
	"periph.io/x/conn/v3/pin"
	"periph.io/x/host/v3/sysfs"
	"strings"
)

// mappingH6 describes the mapping of the H6 processor gpios to their
// alternate functions.
// According to https://linux-sunxi.org/H616 H616 and H618 are the same.
// allwinner marketing team seems to disagree.
//
// It omits the in & out functions which are available on all gpio.
//
// The mapping comes from the datasheet page 26:
// https://linux-sunxi.org/images/3/30/H616_Datasheet_V1.0.pdf
//
//   - The datasheet uses TWI instead of I2C but it is renamed here for
//     consistency.
//   - RGMII means Reduced gigabit media-independent interface.
//   - SDC means SDCard?
//   - NAND connects to a NAND flash controller.
//   - CSI and CCI are for video capture.
//
// The data itself has been extracted from the pdf using
// https://ronnywang.github.io/pdf-table-extractor/ and some
// scripting.  The data has been validated manually.
//
//go:embed H6_pins.json
var h6PinsSpec []byte

type serializedPinSpec struct {
	Name      string
	Function2 pin.Func
	Function3 pin.Func
	Function4 pin.Func
	Function5 pin.Func
	Function6 pin.Func
}

func getH6SerializedPinSpecs() ([]serializedPinSpec, error) {
	var serializedPins []serializedPinSpec
	err := json.Unmarshal(h6PinsSpec, &serializedPins)
	return serializedPins, err
}

func getAltFunc(pinSpec serializedPinSpec) [5]pin.Func {
	return [5]pin.Func{
		pinSpec.Function2,
		pinSpec.Function3,
		pinSpec.Function4,
		pinSpec.Function5,
		pinSpec.Function6}
}

// mapH6Pins uses mappingH6 to actually set the altFunc fields of all gpio
// and mark them as available.
//
// It is called by the generic allwinner processor code if an H6 is detected.
func mapH6Pins() error {
	serializedPinSpecs, err := getH6SerializedPinSpecs()
	if err != nil {
		return err
	}
	for _, pinSpec := range serializedPinSpecs {
		pin := cpupins[pinSpec.Name]
		pin.altFunc = getAltFunc(pinSpec)
		pin.available = true
		if strings.Contains(string(pinSpec.Function6), "_EINT") {
			pin.supportEdge = true
		}
		// Initializes the sysfs corresponding pin right away.
		pin.sysfsPin = sysfs.Pins[pin.Number()]
	}
	return nil
}
