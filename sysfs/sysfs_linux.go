// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Recognize Linux resource-busy errors from sysfs operations.

package sysfs

import (
	"errors"
	"os"
	"syscall"
)

const isLinux = true

func isErrBusy(err error) bool {
	e := &os.PathError{}
	ok := errors.As(err, &e)
	return ok && errors.Is(e.Err, syscall.EBUSY)
}
