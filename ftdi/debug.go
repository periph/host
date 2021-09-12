// Copyright 2021 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:build periph_host_ftdi_debug
// +build periph_host_ftdi_debug

package ftdi

import (
	"log"

	"periph.io/x/d2xx"
	"periph.io/x/d2xx/d2xxtest"
)

// logf is enabled when the build tag host_ftdi_debug is specified.
func logf(fmt string, v ...interface{}) {
	log.Printf(fmt, v...)
}

func (d *driver) resetLog() {
	d.d2xxOpen = func(i int) (d2xx.Handle, d2xx.Err) {
		h, e := d2xx.Open(i)
		if e != 0 {
			return h, e
		}
		return &d2xxtest.Log{H: h, Printf: logf}, e
	}
}
