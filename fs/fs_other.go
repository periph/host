// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Unsupported-platform ioctl and file-event operations.

//go:build !linux

package fs

import "errors"

func ioctl(_ uintptr, _ uint, _ uintptr) error {
	return errors.New("fs: ioctl not supported on non-linux")
}

type event struct{}

func (e *event) makeEvent(_ uintptr) error {
	return errors.New("fs: unreachable code")
}

func (e *event) wait(_ int) (int, error) {
	return 0, errors.New("fs: unreachable code")
}
