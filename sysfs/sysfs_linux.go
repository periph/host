// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Recognize Linux resource-busy errors from sysfs operations.

package sysfs

import (
	"os"
	"syscall"
)

const isLinux = true

func isErrBusy(err error) bool {
	e, ok := err.(*os.PathError)
	return ok && e.Err == syscall.EBUSY
}
