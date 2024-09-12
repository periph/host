//go:build !windows

package netlink

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
