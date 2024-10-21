//go:build !linux

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
//
// Create a dummy chip because ioctl is only supported on Linux

package gpioioctl

func init() {
	if len(Chips) == 0 {
		makeDummyChip()
	}
}
