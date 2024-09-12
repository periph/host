// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
//
// Package gpioioctl provides access to Linux GPIO lines using the ioctl interface.
//
// https://docs.kernel.org/userspace-api/gpio/index.html
//
// GPIO Pins can be accessed via periph.io/x/conn/v3/gpio/gpioreg,
// or using the Chips collection to access the specific GPIO chip
// and using it's ByName()/ByNumber methods.
//
// GPIOChip provides a LineSet feature that allows you to atomically
// read/write to multiple GPIO pins as a single operation.
package gpioioctl
