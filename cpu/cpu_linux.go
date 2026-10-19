// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Linux implementation of short processor busy waits.

package cpu

import (
	"syscall"
	"time"
)

const isLinux = true

func nanospinLinux(d time.Duration) {
	// runtime.nanotime() is not exported so it cannot be used to busy loop for
	// very short sleep (10µs or less).
	time := syscall.NsecToTimespec(d.Nanoseconds())
	leftover := syscall.Timespec{}
	for syscall.Nanosleep(&time, &leftover) != nil {
		time = leftover
	}
}
