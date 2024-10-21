//go:build windows

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
package gpioioctl

import (
	"syscall"
)

const (
	_IOCTL_FUNCTION = 0
)

type Errno uintptr

func syscall_wrapper(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno) { //nolint:unused
	return uintptr(0), uintptr(0), syscall.ERROR_NOT_FOUND
}

func syscall_close_wrapper(fd int) (err error) {
	return syscall.Close(syscall.Handle(fd))
}

func syscall_nonblock_wrapper(fd int, nonblocking bool) (err error) {
	return syscall.SetNonblock(syscall.Handle(fd), nonblocking)
}
