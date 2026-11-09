//go:build windows

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Non-Linux syscall shims for the GPIO character-device implementation.

package gpioioctl

import (
	"syscall"
)

const (
	ioctlFunction = 0
)

// Errno represents an operating system error number.
type Errno uintptr

func syscallWrapper(_, _, _, _ uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return uintptr(0), uintptr(0), syscall.ERROR_NOT_FOUND
}

func syscallCloseWrapper(fd int) (err error) {
	return syscall.Close(syscall.Handle(fd))
}

func syscallNonblockWrapper(fd int, nonblocking bool) (err error) {
	return syscall.SetNonblock(syscall.Handle(fd), nonblocking)
}
