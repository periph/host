// Copyright 2022 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// nanoPi pin out.

package nanopi

import (
	"errors"
	"fmt"
	"strings"

	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/pin"
	"periph.io/x/conn/v3/pin/pinreg"
	"periph.io/x/host/v3/allwinner"
	"periph.io/x/host/v3/distro"
)

// Present return true if a NanoPi board is detected.
func Present() bool {
	if isArm {
		// This works for the NanoPi Neo Air, not sure if other NAnoPi boards
		// match the same DTModel prefix.
		return strings.HasPrefix(distro.DTModel(), "FriendlyARM")
	}

	return false
}

const (
	boardNeoAir string = "NanoPi NEO Air" // + LTS (H2/H3 have identical pinouts)
)

var (
	PA1_1  pin.Pin    = pin.DC_IN // VCC 3v3 Ext
	PA1_2  pin.Pin    = pin.V5
	PA1_3  gpio.PinIO = allwinner.PA12
	PA1_4  pin.Pin    = pin.V5
	PA1_5  gpio.PinIO = allwinner.PA11
	PA1_6  pin.Pin    = pin.GROUND
	PA1_7  gpio.PinIO = allwinner.PG11
	PA1_8  gpio.PinIO = allwinner.PG6
	PA1_9  pin.Pin    = pin.GROUND
	PA1_10 gpio.PinIO = allwinner.PG7
	PA1_11 gpio.PinIO = allwinner.PA0
	PA1_12 gpio.PinIO = allwinner.PA6
	PA1_13 gpio.PinIO = allwinner.PA2
	PA1_14 pin.Pin    = pin.GROUND
	PA1_15 gpio.PinIO = allwinner.PA3
	PA1_16 gpio.PinIO = allwinner.PA8
	PA1_17 pin.Pin    = pin.DC_IN // VCC 3v3 Ext
	PA1_18 gpio.PinIO = allwinner.PG9
	PA1_19 gpio.PinIO = allwinner.PC0
	PA1_20 pin.Pin    = pin.GROUND
	PA1_21 gpio.PinIO = allwinner.PC1
	PA1_22 gpio.PinIO = allwinner.PA1
	PA1_23 gpio.PinIO = allwinner.PC2
	PA1_24 gpio.PinIO = allwinner.PC3

	FUN1_1  pin.Pin = pin.V5
	FUN1_2  pin.Pin = gpio.INVALID   // USB-DP2
	FUN1_3  pin.Pin = gpio.INVALID   // USB-DM2
	FUN1_4  pin.Pin = gpio.INVALID   // USB-DP3
	FUN1_5  pin.Pin = gpio.INVALID   // USB-DM3
	FUN1_6  pin.Pin = allwinner.PL11 // IR-RX
	FUN1_7  pin.Pin = allwinner.PA17 // SPDIF-OUT
	FUN1_8  pin.Pin = allwinner.PA18 // PCM0_SYNC/I2S0_LRC
	FUN1_9  pin.Pin = allwinner.PA19 // PCM0_CLK/I2S0_BCK
	FUN1_10 pin.Pin = allwinner.PA20 // PCM0_DOUT/I2S0_SDOUT
	FUN1_11 pin.Pin = allwinner.PA21 // PCM0_DIN/I2S0_SDIN
	FUN1_12 pin.Pin = pin.GROUND     //

	FUN2_1 pin.Pin = allwinner.HP_LEFT  // LINEOUTL
	FUN2_2 pin.Pin = allwinner.HP_RIGHT // LINEOUT
	FUN2_3 pin.Pin = allwinner.MIC_GND  // MIC GND
	FUN2_4 pin.Pin = allwinner.MIC_IN   // MIC IN

	FUN3_1 pin.Pin = pin.GROUND
	FUN3_2 pin.Pin = pin.V5
	FUN3_3 pin.Pin = allwinner.PA4 // UART TX
	FUN3_4 pin.Pin = allwinner.PA5 // UART RX

)

// registerHeaders registers the headers for various NanoPi boards. Currently
// only NanoPi NEO Air is supported.
func registerHeaders(model string) error {
	if strings.Contains(model, boardNeoAir) {
		// 24pin expansion port
		if err := pinreg.Register("PA", [][]pin.Pin{
			{PA1_1, PA1_2},
			{PA1_3, PA1_4},
			{PA1_5, PA1_6},
			{PA1_7, PA1_8},
			{PA1_9, PA1_10},
			{PA1_11, PA1_12},
			{PA1_13, PA1_14},
			{PA1_15, PA1_16},
			{PA1_17, PA1_18},
			{PA1_19, PA1_20},
			{PA1_21, PA1_22},
			{PA1_23, PA1_24},
		}); err != nil {
			return err
		}

		// 12pin function interface
		if err := pinreg.Register("FUN", [][]pin.Pin{
			{FUN1_1},
			{FUN1_2},
			{FUN1_3},
			{FUN1_4},
			{FUN1_5},
			{FUN1_6},
			{FUN1_7},
			{FUN1_8},
			{FUN1_9},
			{FUN1_10},
			{FUN1_11},
			{FUN1_12},
			{FUN2_1},
			{FUN2_2},
			{FUN2_3},
			{FUN2_4},
			{FUN3_1},
			{FUN3_2},
			{FUN3_3},
			{FUN3_4},
		}); err != nil {
			return err
		}
	}

	return nil
}

// driver implements periph.Driver.
type driver struct {
}

// String is the text representation of the board.
func (d *driver) String() string {
	return "nanopi"
}

// Prerequisites load drivers before the actual driver is loaded. For
// these boards, we do not need any prerequisites.
func (d *driver) Prerequisites() []string {
	return nil
}

// After this driver is loaded, we need to load generic Allwinner drivers
// for the GPIO pins which are identical on all Allwinner CPUs.
func (d *driver) After() []string {
	return []string{"allwinner-gpio", "allwinner-gpio-pl"}
}

// Init initializes the driver by checking its presence and if found, the
// driver will be registered.
func (d *driver) Init() (bool, error) {
	if !Present() {
		return false, errors.New("NanoPi board not detected")
	}

	model := distro.DTModel()
	if model == "<unknown>" {
		return true, fmt.Errorf("nanopi: failed to obtain model")
	}

	err := registerHeaders(model)
	return true, err
}

// init register the driver.
func init() {
	if isArm {
		driverreg.MustRegister(&drv)
	}
}

var drv driver
