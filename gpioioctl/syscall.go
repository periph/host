//go:build !windows

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
//
// This file provides a wrapper around syscall so that we can have the same source
// for Windows/Linux. The problem is that in Linux syscall.Syscall takes a unintptr
// as the first arg, while on windows it's a syscall.Handle. It also handles
// syscall.SYS_IOCTL not being defined things besides linux.

package gpioioctl

import (
	"syscall"
)

const (
	_IOCTL_FUNCTION = syscall.SYS_IOCTL
)

func syscall_wrapper(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return syscall.Syscall(trap, a1, a2, a3)
}

func syscall_close_wrapper(fd int) (err error) {
	return syscall.Close(fd)
}

func syscall_nonblock_wrapper(fd int, nonblocking bool) (err error) {
	return syscall.SetNonblock(fd, nonblocking)
}
