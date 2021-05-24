// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ftdi

import (
	"errors"
	"fmt"
	"unsafe"
)

// EEPROM is the unprocessed EEPROM content.
//
// The EEPROM is in 3 parts: the defined struct, the 4 strings and the rest
// which is used as an 'user area'. The size of the user area depends on the
// length of the strings. The user area content is not included in this struct.
type EEPROM struct {
	// Raw is the raw EEPROM content. It excludes the strings.
	Raw []byte

	// The following condition must be true: len(Manufacturer) + len(Desc) <= 40.
	Manufacturer   string
	ManufacturerID string
	Desc           string
	Serial         string
}

// Validate checks that the data is good.
func (e *EEPROM) Validate() error {
	// Verify that the values are set correctly.
	if len(e.Manufacturer) > 40 {
		return errors.New("ftdi: Manufacturer is too long")
	}
	if len(e.ManufacturerID) > 40 {
		return errors.New("ftdi: ManufacturerID is too long")
	}
	if len(e.Desc) > 40 {
		return errors.New("ftdi: Desc is too long")
	}
	if len(e.Serial) > 40 {
		return errors.New("ftdi: Serial is too long")
	}
	if len(e.Manufacturer)+len(e.Desc) > 40 {
		return errors.New("ftdi: length of Manufacturer plus Desc is too long")
	}
	return nil
}

func (e *EEPROM) AsHeader() *EEPROMHeader {
	// sizeof(EEPROMHeader)
	if len(e.Raw) < 16 {
		return nil
	}
	return (*EEPROMHeader)(unsafe.Pointer(&e.Raw[0]))
}

// AsFT232H returns the Raw data aliased as EEPROMFT232H.
func (e *EEPROM) AsFT232H() *EEPROMFT232H {
	// sizeof(EEPROMFT232H)
	if len(e.Raw) < 44 {
		return nil
	}
	return (*EEPROMFT232H)(unsafe.Pointer(&e.Raw[0]))
}

// AsFT2232H returns the Raw data aliased as EEPROMFT2232H.
func (e *EEPROM) AsFT2232H() *EEPROMFT2232H {
	// sizeof(EEPROMFT2232H)
	if len(e.Raw) < 40 {
		return nil
	}
	return (*EEPROMFT2232H)(unsafe.Pointer(&e.Raw[0]))
}

// AsFT232R returns the Raw data aliased as EEPROMFT232R.
func (e *EEPROM) AsFT232R() *EEPROMFT232R {
	// sizeof(EEPROMFT232R)
	if len(e.Raw) < 32 {
		return nil
	}
	return (*EEPROMFT232R)(unsafe.Pointer(&e.Raw[0]))
}

// FT232hCBusMux is stored in the FT232H EEPROM to control each CBus pin.
type FT232hCBusMux uint8

const (
	// TriSt-PU; Sets in Tristate (pull up) (C0~C6, C8, C9) on 75kΩ.
	FT232hCBusTristatePullUp FT232hCBusMux = 0x00
	// TXLED#; Pulses low when transmitting data (C0~C6, C8, C9).
	FT232hCBusTxLED FT232hCBusMux = 0x01
	// RXLED#; Pulses low when receiving data (C0~C6, C8, C9).
	FT232hCBusRxLED FT232hCBusMux = 0x02
	// TX&RXLED#; Pulses low when either receiving or transmitting data (C0~C6,
	// C8, C9).
	FT232hCBusTxRxLED FT232hCBusMux = 0x03
	// PWREN#; Output is low after the device has been configured by USB, then
	// high during USB suspend mode (C0~C6, C8, C9).
	//
	// Must be used with an external 10kΩ pull up.
	FT232hCBusPwrEnable FT232hCBusMux = 0x04
	// SLEEP#; Goes low during USB suspend mode (C0~C6, C8, C9).
	FT232hCBusSleep FT232hCBusMux = 0x05
	// DRIVE1; Drives pin to logic 0 (C0~C6, C8, C9).
	FT232hCBusDrive0 FT232hCBusMux = 0x06
	// DRIVE1; Drives pin to logic 1 (C0, C5, C6, C8, C9).
	FT232hCBusDrive1 FT232hCBusMux = 0x07
	// I/O Mode; CBus bit-bang mode option (C5, C6, C8, C9).
	FT232hCBusIOMode FT232hCBusMux = 0x08
	// TXDEN; Tx Data Enable. Used with RS485 level converters to enable the line
	// driver during data transmit. It is active one bit time before the start
	// bit up to until the end of the stop bit (C0~C6, C8, C9).
	FT232hCBusTxdEnable FT232hCBusMux = 0x09
	// CLK30 30MHz clock output (C0, C5, C6, C8, C9).
	FT232hCBusClk30 FT232hCBusMux = 0x0A
	// CLK15 15MHz clock output (C0, C5, C6, C8, C9).
	FT232hCBusClk15 FT232hCBusMux = 0x0B
	// CLK7.5 7.5MHz clock output (C0, C5, C6, C8, C9).
	FT232hCBusClk7_5 FT232hCBusMux = 0x0C
)

const ft232hCBusMuxName = "FT232hCBusTristatePullUpFT232hCBusTxLEDFT232hCBusRxLEDFT232hCBusTxRxLEDFT232hCBusPwrEnableFT232hCBusSleepFT232hCBusDrive0FT232hCBusDrive1FT232hCBusIOModeFT232hCBusTxdEnableFT232hCBusClk30FT232hCBusClk15FT232hCBusClk7_5"

var fr232hCBusMuxIndex = [...]uint8{0, 24, 39, 54, 71, 90, 105, 121, 137, 153, 172, 187, 202, 218}

func (f FT232hCBusMux) String() string {
	if f >= FT232hCBusMux(len(fr232hCBusMuxIndex)-1) {
		return fmt.Sprintf("FT232hCBusMux(%d)", f)
	}
	return ft232hCBusMuxName[fr232hCBusMuxIndex[f]:fr232hCBusMuxIndex[f+1]]
}

// FT232rCBusMux is stored in the FT232R EEPROM to control each CBus pin.
type FT232rCBusMux uint8

const (
	// TXDEN; Tx Data Enable. Used with RS485 level converters to enable the line
	// driver during data transmit. It is active one bit time before the start
	// bit up to until the end of the stop bit (C0~C4).
	FT232rCBusTxdEnable FT232rCBusMux = 0x00
	// PWREN#; Output is low after the device has been configured by USB, then
	// high during USB suspend mode (C0~C4).
	//
	// Must be used with an external 10kΩ pull up.
	FT232rCBusPwrEnable FT232rCBusMux = 0x01
	// RXLED#; Pulses low when receiving data (C0~C4).
	FT232rCBusRxLED FT232rCBusMux = 0x02
	// TXLED#; Pulses low when transmitting data (C0~C4).
	FT232rCBusTxLED FT232rCBusMux = 0x03
	// TX&RXLED#; Pulses low when either receiving or transmitting data (C0~C4).
	FT232rCBusTxRxLED FT232rCBusMux = 0x04
	// SLEEP# Goes low during USB suspend mode (C0~C4).
	FT232rCBusSleep FT232rCBusMux = 0x05
	// CLK48 48Mhz +/-0.7% clock output (C0~C4).
	FT232rCBusClk48 FT232rCBusMux = 0x06
	// CLK24 24Mhz clock output (C0~C4).
	FT232rCBusClk24 FT232rCBusMux = 0x07
	// CLK12 12Mhz clock output (C0~C4).
	FT232rCBusClk12 FT232rCBusMux = 0x08
	// CLK6 6Mhz +/-0.7% clock output (C0~C4).
	FT232rCBusClk6 FT232rCBusMux = 0x09
	// CBitBangI/O; CBus bit-bang mode option (C0~C3).
	FT232rCBusIOMode FT232rCBusMux = 0x0A
	// BitBangWRn; CBus WR# strobe output (C0~C3).
	FT232rCBusBitBangWR FT232rCBusMux = 0x0B
	// BitBangRDn; CBus RD# strobe output (C0~C3).
	FT232rCBusBitBangRD FT232rCBusMux = 0x0C
)

const ft232rCBusMuxName = "FT232rCBusTxdEnableFT232rCBusPwrEnableFT232rCBusRxLEDFT232rCBusTxLEDFT232rCBusTxRxLEDFT232rCBusSleepFT232rCBusClk48FT232rCBusClk24FT232rCBusClk12FT232rCBusClk6FT232rCBusIOModeFT232rCBusBitBangWRFT232rCBusBitBangRD"

var ft232rCBusMuxIndex = [...]uint8{0, 19, 38, 53, 68, 85, 100, 115, 130, 145, 159, 175, 194, 213}

func (f FT232rCBusMux) String() string {
	if f >= FT232rCBusMux(len(ft232rCBusMuxIndex)-1) {
		return fmt.Sprintf("FT232rCBusMux(%d)", f)
	}
	return ft232rCBusMuxName[ft232rCBusMuxIndex[f]:ft232rCBusMuxIndex[f+1]]
}

// EEPROMHeader is the common header found on FTDI devices.
//
// It is 16 bytes long.
type EEPROMHeader struct {
	DeviceType     DevType // 0x00 FTxxxx device type to be programmed
	VendorID       uint16  // 0x04 Defaults to 0x0403; can be changed
	ProductID      uint16  // 0x06 Defaults to 0x6001 for FT232R, 0x6014 for FT232H, relevant value
	SerNumEnable   uint8   // 0x07 bool Non-zero if serial number to be used
	Unused0        uint8   // 0x08 For alignment.
	MaxPower       uint16  // 0x0A 0mA < MaxPower <= 500mA
	SelfPowered    uint8   // 0x0C bool 0 = bus powered, 1 = self powered
	RemoteWakeup   uint8   // 0x0D bool 0 = not capable, 1 = capable; RI# low will wake host in 20ms.
	PullDownEnable uint8   // 0x0E bool Non zero if pull down in suspend enabled
	Unused1        uint8   // 0x0F For alignment.
}

// EEPROMFT232H is the EEPROM layout of a FT232H device.
//
// It is 44 bytes long.
type EEPROMFT232H struct {
	EEPROMHeader

	// FT232H specific.
	ACSlowSlew        uint8         // 0x10 bool Non-zero if AC bus pins have slow slew
	ACSchmittInput    uint8         // 0x11 bool Non-zero if AC bus pins are Schmitt input
	ACDriveCurrent    uint8         // 0x12 Valid values are 4mA, 8mA, 12mA, 16mA in 2mA units
	ADSlowSlew        uint8         // 0x13 bool Non-zero if AD bus pins have slow slew
	ADSchmittInput    uint8         // 0x14 bool Non-zero if AD bus pins are Schmitt input
	ADDriveCurrent    uint8         // 0x15 Valid values are 4mA, 8mA, 12mA, 16mA in 2mA units
	Cbus0             FT232hCBusMux // 0x16
	Cbus1             FT232hCBusMux // 0x17
	Cbus2             FT232hCBusMux // 0x18
	Cbus3             FT232hCBusMux // 0x19
	Cbus4             FT232hCBusMux // 0x1A
	Cbus5             FT232hCBusMux // 0x1B
	Cbus6             FT232hCBusMux // 0x1C
	Cbus7             FT232hCBusMux // 0x1D C7 is limited a sit can only do 'suspend on C7 low'. Defaults pull down.
	Cbus8             FT232hCBusMux // 0x1E
	Cbus9             FT232hCBusMux // 0x1F
	FT1248Cpol        uint8         // 0x20 bool FT1248 clock polarity - clock idle high (true) or clock idle low (false)
	FT1248Lsb         uint8         // 0x21 bool FT1248 data is LSB (true), or MSB (false)
	FT1248FlowControl uint8         // 0x22 bool FT1248 flow control enable
	IsFifo            uint8         // 0x23 bool Non-zero if Interface is 245 FIFO
	IsFifoTar         uint8         // 0x24 bool Non-zero if Interface is 245 FIFO CPU target
	IsFastSer         uint8         // 0x25 bool Non-zero if Interface is Fast serial
	IsFT1248          uint8         // 0x26 bool Non-zero if Interface is FT1248
	PowerSaveEnable   uint8         // 0x27 bool Suspect on ACBus7 low.
	DriverType        uint8         // 0x28 bool 0 is D2XX, 1 is VCP
	Unused2           uint8         // 0x29
	Unused3           uint16        // 0x30
}

func (e *EEPROMFT232H) Defaults() {
	// As found on Adafruit device.
	e.ACDriveCurrent = 4
	e.ADDriveCurrent = 4
	e.Cbus0 = FT232hCBusTristatePullUp
	e.Cbus1 = FT232hCBusTristatePullUp
	e.Cbus2 = FT232hCBusTristatePullUp
	e.Cbus3 = FT232hCBusTristatePullUp
	e.Cbus4 = FT232hCBusTristatePullUp
	e.Cbus5 = FT232hCBusTristatePullUp
	e.Cbus6 = FT232hCBusTristatePullUp
	e.Cbus7 = FT232hCBusTristatePullUp
	e.Cbus8 = FT232hCBusDrive1
	e.Cbus9 = FT232hCBusDrive0
}

// EEPROMFT2232H is the EEPROM layout of a FT2232H device.
//
// It is 40 bytes long.
type EEPROMFT2232H struct {
	EEPROMHeader

	// FT232H specific.
	ALSlowSlew      uint8  // 0x10 bool non-zero if AL pins have slow slew
	ALSchmittInput  uint8  // 0x11 bool non-zero if AL pins are Schmitt input
	ALDriveCurrent  uint8  // 0x12 Valid values are 4mA, 8mA, 12mA, 16mA in 2mA units
	AHSlowSlew      uint8  // 0x13 bool non-zero if AH pins have slow slew
	AHSchmittInput  uint8  // 0x14 bool non-zero if AH pins are Schmitt input
	AHDriveCurrent  uint8  // 0x15 Valid values are 4mA, 8mA, 12mA, 16mA in 2mA units
	BLSlowSlew      uint8  // 0x16 bool non-zero if BL pins have slow slew
	BLSchmittInput  uint8  // 0x17 bool non-zero if BL pins are Schmitt input
	BLDriveCurrent  uint8  // 0x18 Valid values are 4mA, 8mA, 12mA, 16mA in 2mA units
	BHSlowSlew      uint8  // 0x19 bool non-zero if BH pins have slow slew
	BHSchmittInput  uint8  // 0x1A bool non-zero if BH pins are Schmitt input
	BHDriveCurrent  uint8  // 0x1B Valid values are 4mA, 8mA, 12mA, 16mA in 2mA units
	AIsFifo         uint8  // 0x1C bool non-zero if interface is 245 FIFO
	AIsFifoTar      uint8  // 0x1D bool non-zero if interface is 245 FIFO CPU target
	AIsFastSer      uint8  // 0x1E bool non-zero if interface is Fast serial
	BIsFifo         uint8  // 0x1F bool non-zero if interface is 245 FIFO
	BIsFifoTar      uint8  // 0x20 bool non-zero if interface is 245 FIFO CPU target
	BIsFastSer      uint8  // 0x21 bool non-zero if interface is Fast serial
	PowerSaveEnable uint8  // 0x22 bool non-zero if using BCBUS7 to save power for self-powered designs
	ADriverType     uint8  // 0x23 bool
	BDriverType     uint8  // 0x24 bool
	Unused2         uint8  // 0x25
	Unused3         uint16 // 0x26
}

// EEPROMFT232R is the EEPROM layout of a FT232R device.
//
// It is 32 bytes long.
type EEPROMFT232R struct {
	EEPROMHeader

	// FT232R specific.
	IsHighCurrent uint8         // 0x10 bool High Drive I/Os; 3mA instead of 1mA (@3.3V)
	UseExtOsc     uint8         // 0x11 bool Use external oscillator
	InvertTXD     uint8         // 0x12 bool
	InvertRXD     uint8         // 0x13 bool
	InvertRTS     uint8         // 0x14 bool
	InvertCTS     uint8         // 0x15 bool
	InvertDTR     uint8         // 0x16 bool
	InvertDSR     uint8         // 0x17 bool
	InvertDCD     uint8         // 0x18 bool
	InvertRI      uint8         // 0x19 bool
	Cbus0         FT232rCBusMux // 0x1A Default ft232rCBusTxLED
	Cbus1         FT232rCBusMux // 0x1B Default ft232rCBusRxLED
	Cbus2         FT232rCBusMux // 0x1C Default ft232rCBusTxdEnable
	Cbus3         FT232rCBusMux // 0x1D Default ft232rCBusPwrEnable
	Cbus4         FT232rCBusMux // 0x1E Default ft232rCBusSleep
	DriverType    uint8         // 0x1F bool 0 is D2XX, 1 is VCP
}

func (e *EEPROMFT232R) Defaults() {
	// As found on Adafruit device.
	e.Cbus0 = FT232rCBusTxLED
	e.Cbus1 = FT232rCBusRxLED
	e.Cbus2 = FT232rCBusTxdEnable
	e.Cbus3 = FT232rCBusPwrEnable
	e.Cbus4 = FT232rCBusSleep
	e.DriverType = 1
}

//

// DevType is the FTDI device type.
type DevType uint32

const (
	DevTypeFTBM DevType = iota // 0
	DevTypeFTAM
	DevTypeFT100AX
	DevTypeUnknown // 3
	DevTypeFT2232C
	DevTypeFT232R // 5
	DevTypeFT2232H
	DevTypeFT4232H
	DevTypeFT232H // 8
	DevTypeFTXSeries
	DevTypeFT4222H0
	DevTypeFT4222H1_2
	DevTypeFT4222H3
	DevTypeFT4222Prog
	DevTypeFT900
	DevTypeFT930
	DevTypeFTUMFTPD3A
)

// EEPROMSize returns the size of the EEPROM for this device.
func (d DevType) EEPROMSize() int {
	switch d {
	case DevTypeFT232H:
		// sizeof(EEPROMFT232H)
		return 44
	case DevTypeFT2232H:
		// sizeof(EEPROMFT2232H)
		return 40
	case DevTypeFT232R:
		// sizeof(EEPROMFT232R)
		return 32
	default:
		return 256
	}
}

const devTypeName = "FTBMFTAMFT100AXUnknownFT2232CFT232RFT2232HFT4232HFT232HFTXSeriesFT4222H0FT4222H1/2FT4222H3FT4222ProgFT900FT930FTUMFTPD3A"

var devTypeIndex = [...]uint8{0, 4, 8, 15, 22, 29, 35, 42, 49, 55, 64, 72, 82, 90, 100, 105, 110, 120}

func (d DevType) String() string {
	if d >= DevType(len(devTypeIndex)-1) {
		d = DevTypeUnknown
	}
	return devTypeName[devTypeIndex[d]:devTypeIndex[d+1]]
}
