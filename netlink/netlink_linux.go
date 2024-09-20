// Copyright 2019 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package netlink

import (
	"fmt"
	"path/filepath"
	"syscall"
)

const isLinux = true

// connSocket is a simple wrapper around a Linux netlink connector socket.
type connSocket struct {
	fd int
}

// newConnSocket returns a socket instance.
func newConnSocket() (*connSocket, error) {
	// Open netlink socket.
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_CONNECTOR)
	if err != nil {
		return nil, fmt.Errorf("failed to open netlink socket: %v", err)
	}

	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return nil, fmt.Errorf("failed to bind netlink socket: %v", err)
	}

	return &connSocket{fd: fd}, nil
}

// send writes w to the socket.
func (s *connSocket) send(w []byte) error {
	return syscall.Sendto(s.fd, w, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK})
}

// recv reads at most len(r) bytes from the socket into r. Returns the actually
// read number of bytes.
func (s *connSocket) recv(r []byte) (int, error) {
	n, _, err := syscall.Recvfrom(s.fd, r, 0)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// close closes the socket.
func (s *connSocket) close() error {
	fd := s.fd
	s.fd = 0
	return syscall.Close(fd)
}

// isOneWireAvailable checks to see if the Linux onewire bus drivers are loaded.
// It does this by looking for entries in the /sys/bus pseudo-filesystem.
//
// On a Raspberry Pi SBC, the onewire bus is enabled by creating entries in the
// kernel config.txt file which is located in /boot, or /boot/firmware depending
// on OS/kernel levels.
func isOneWireAvailable() bool {
	items, err := filepath.Glob("/sys/bus/w1/devices/*")
	if err != nil || len(items) == 0 {
		return false
	}
	return true
}
