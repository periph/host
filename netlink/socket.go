//go:build !windows

package netlink

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// connSocket is a simple wrapper around a Linux netlink connector socket.
type connSocket struct {
	fd int
}

// w1Socket is a netlink connector socket for communicating with the w1 Linux
// kernel module.
type w1Socket struct {
	s  socket
	fd int
}
