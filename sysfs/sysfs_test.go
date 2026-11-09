// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Shared fake sysfs files, ioctl handles, and driver test setup.

package sysfs

import (
	"errors"
	"io"

	"periph.io/x/host/v3/fs"
)

func init() {
	fs.Inhibit()
	reset()
}

func reset() {
	fileIOOpen = fileIOOpenDefault
	ioctlOpen = ioctlOpenDefault
	// Soon.
	// fileIOOpen = fileIOOpenPanic
	// ioctlOpen = ioctlOpenPanic
}

func ioctlOpenPanic(_ string, _ int) (ioctlCloser, error) {
	panic("don't forget to override fileIOOpen")
}

func fileIOOpenPanic(_ string, _ int) (fileIO, error) {
	panic("don't forget to override fileIOOpen")
}

type ioctlClose struct {
	ioctlErr error
	closeErr error
}

func (i *ioctlClose) Ioctl(_ uint, _ uintptr) error {
	return i.ioctlErr
}

func (i *ioctlClose) Close() error {
	return i.closeErr
}

type file struct {
	ioctlClose
}

func (f *file) Fd() uintptr {
	return 0xFFFFFFFF
}

func (f *file) Read(_ []byte) (int, error) {
	return 0, errors.New("not implemented")
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 && whence == io.SeekStart {
		return 0, nil
	}
	return 0, errors.New("not implemented")
}

func (f *file) Write(_ []byte) (int, error) {
	return 0, errors.New("not implemented")
}
