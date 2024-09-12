package netlink

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

import (
	"syscall"
)

// connSocket is a simple wrapper around a Linux netlink connector socket.
type connSocket struct {
	fd syscall.Handle
}

// w1Socket is a netlink connector socket for communicating with the w1 Linux
// kernel module.
type w1Socket struct {
	s  socket
	fd syscall.Handle
}
