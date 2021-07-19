// Copyright 2021 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build !periph_host_ftdi_debug

package ftdi

// logf is disabled when the build tag host_ftdi_debug is not specified.
func logf(fmt string, v ...interface{}) {
}

func (d *driver) resetLog() {
}
