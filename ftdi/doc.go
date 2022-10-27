// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package ftdi implements support for popular FTDI devices.
//
// The supported devices (FT232h/FT232r) implement support for various
// protocols like the GPIO, I²C, SPI, UART, JTAG.
//
// Use build tag periph_host_ftdi_debug to enable verbose debugging.
//
// # More details
//
// See https://periph.io/device/ftdi/ for more details, and how to configure
// the host to be able to use this driver.
//
// # Datasheets
//
// http://www.ftdichip.com/Support/Documents/DataSheets/ICs/DS_FT232R.pdf
//
// http://www.ftdichip.com/Support/Documents/DataSheets/ICs/DS_FT232H.pdf
package ftdi
