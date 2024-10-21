package gpioioctl

// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// This file contains definitions and methods for using the GPIO IOCTL calls.
//
// Documentation for the ioctl() API is at:
//
// https://docs.kernel.org/userspace-api/gpio/index.html

import (
	"errors"
	"unsafe"
)

// From the linux /usr/include/asm-generic/ioctl.h file.
const (
	_IOC_NONE  = 0
	_IOC_WRITE = 1
	_IOC_READ  = 2

	_IOC_NRBITS   = 8
	_IOC_TYPEBITS = 8
	_IOC_SIZEBITS = 14

	_IOC_NRSHIFT   = 0
	_IOC_TYPESHIFT = _IOC_NRSHIFT + _IOC_NRBITS
	_IOC_SIZESHIFT = _IOC_TYPESHIFT + _IOC_TYPEBITS
	_IOC_DIRSHIFT  = _IOC_SIZESHIFT + _IOC_SIZEBITS
)

func _IOC(dir, typ, nr, size uintptr) uintptr {
	return dir<<_IOC_DIRSHIFT |
		typ<<_IOC_TYPESHIFT |
		nr<<_IOC_NRSHIFT |
		size<<_IOC_SIZESHIFT
}

func _IOR(typ, nr, size uintptr) uintptr {
	return _IOC(_IOC_READ, typ, nr, size)
}

func _IOWR(typ, nr, size uintptr) uintptr {
	return _IOC(_IOC_READ|_IOC_WRITE, typ, nr, size)
}

// From the /usr/include/linux/gpio.h header file.
const (
	_GPIO_MAX_NAME_SIZE         = 32
	_GPIO_V2_LINE_NUM_ATTRS_MAX = 10
	_GPIO_V2_LINES_MAX          = 64

	_GPIO_V2_LINE_FLAG_USED                 uint64 = 1 << 0
	_GPIO_V2_LINE_FLAG_ACTIVE_LOW           uint64 = 1 << 1
	_GPIO_V2_LINE_FLAG_INPUT                uint64 = 1 << 2
	_GPIO_V2_LINE_FLAG_OUTPUT               uint64 = 1 << 3
	_GPIO_V2_LINE_FLAG_EDGE_RISING          uint64 = 1 << 4
	_GPIO_V2_LINE_FLAG_EDGE_FALLING         uint64 = 1 << 5
	_GPIO_V2_LINE_FLAG_OPEN_DRAIN           uint64 = 1 << 6
	_GPIO_V2_LINE_FLAG_OPEN_SOURCE          uint64 = 1 << 7
	_GPIO_V2_LINE_FLAG_BIAS_PULL_UP         uint64 = 1 << 8
	_GPIO_V2_LINE_FLAG_BIAS_PULL_DOWN       uint64 = 1 << 9
	_GPIO_V2_LINE_FLAG_BIAS_DISABLED        uint64 = 1 << 10
	_GPIO_V2_LINE_FLAG_EVENT_CLOCK_REALTIME uint64 = 1 << 11
	_GPIO_V2_LINE_FLAG_EVENT_CLOCK_HTE      uint64 = 1 << 12

	_GPIO_V2_LINE_EVENT_RISING_EDGE  uint32 = 1
	_GPIO_V2_LINE_EVENT_FALLING_EDGE uint32 = 2

	_GPIO_V2_LINE_ATTR_ID_FLAGS         uint32 = 1
	_GPIO_V2_LINE_ATTR_ID_OUTPUT_VALUES uint32 = 2
	_GPIO_V2_LINE_ATTR_ID_DEBOUNCE      uint32 = 3
)

type gpiochip_info struct {
	name  [_GPIO_MAX_NAME_SIZE]byte
	label [_GPIO_MAX_NAME_SIZE]byte
	lines uint32
}

type gpio_v2_line_attribute struct {
	id      uint32
	padding uint32
	// value is actually a union who's interpretation is dependent upon
	// the value of id.
	value uint64
}

type gpio_v2_line_config_attribute struct {
	attr gpio_v2_line_attribute

	mask uint64
}

type gpio_v2_line_config struct {
	flags     uint64
	num_attrs uint32
	padding   [5]uint32
	attrs     [_GPIO_V2_LINE_NUM_ATTRS_MAX]gpio_v2_line_config_attribute
}

type gpio_v2_line_request struct {
	offsets           [_GPIO_V2_LINES_MAX]uint32
	consumer          [_GPIO_MAX_NAME_SIZE]byte
	config            gpio_v2_line_config
	num_lines         uint32
	event_buffer_size uint32
	padding           [5]uint32
	fd                int32
}

// setLineNumber works around the false positive in gosec for using copy
func (lr *gpio_v2_line_request) setLineNumber(element int, number uint32) {
	lr.offsets[element] = number
}

type gpio_v2_line_values struct {
	bits uint64
	mask uint64
}

type gpio_v2_line_info struct {
	name      [_GPIO_MAX_NAME_SIZE]byte
	consumer  [_GPIO_MAX_NAME_SIZE]byte
	offset    uint32
	num_attrs uint32
	flags     uint64
	attrs     [_GPIO_V2_LINE_NUM_ATTRS_MAX]gpio_v2_line_attribute
	padding   [4]uint32
}

type gpio_v2_line_event struct {
	Timestamp_ns uint64
	Id           uint32
	Offset       uint32
	Seqno        uint32
	LineSeqno    uint32
	Padding      [6]uint32
}

func ioctl_get_gpio_v2_line_values(fd uintptr, data *gpio_v2_line_values) error {
	arg := _IOWR(0xb4, 0x0e, unsafe.Sizeof(gpio_v2_line_values{}))
	_, _, ep := syscall_wrapper(_IOCTL_FUNCTION, fd, arg, uintptr(unsafe.Pointer(data)))
	if ep != 0 {
		return errors.New(ep.Error())
	}
	return nil
}

func ioctl_set_gpio_v2_line_values(fd uintptr, data *gpio_v2_line_values) error {
	arg := _IOWR(0xb4, 0x0f, unsafe.Sizeof(gpio_v2_line_values{}))
	_, _, ep := syscall_wrapper(_IOCTL_FUNCTION, fd, arg, uintptr(unsafe.Pointer(data)))
	if ep != 0 {
		return errors.New(ep.Error())
	}
	return nil
}
func ioctl_gpiochip_info(fd uintptr, data *gpiochip_info) error {
	arg := _IOR(0xb4, 0x01, unsafe.Sizeof(gpiochip_info{}))
	_, _, ep := syscall_wrapper(_IOCTL_FUNCTION, fd, arg, uintptr(unsafe.Pointer(data)))
	if ep != 0 {
		return errors.New(ep.Error())
	}
	return nil
}

func ioctl_gpio_v2_line_info(fd uintptr, data *gpio_v2_line_info) error {
	arg := _IOWR(0xb4, 0x05, unsafe.Sizeof(gpio_v2_line_info{}))
	_, _, ep := syscall_wrapper(_IOCTL_FUNCTION, fd, arg, uintptr(unsafe.Pointer(data)))
	if ep != 0 {
		return errors.New(ep.Error())
	}
	return nil
}

func ioctl_gpio_v2_line_config(fd uintptr, data *gpio_v2_line_config) error {
	arg := _IOWR(0xb4, 0x0d, unsafe.Sizeof(gpio_v2_line_config{}))
	_, _, ep := syscall_wrapper(_IOCTL_FUNCTION, fd, arg, uintptr(unsafe.Pointer(data)))
	if ep != 0 {
		return errors.New(ep.Error())
	}
	return nil
}

func ioctl_gpio_v2_line_request(fd uintptr, data *gpio_v2_line_request) error {
	arg := _IOWR(0xb4, 0x07, unsafe.Sizeof(gpio_v2_line_request{}))
	_, _, ep := syscall_wrapper(_IOCTL_FUNCTION, fd, arg, uintptr(unsafe.Pointer(data)))
	if ep != 0 {
		return errors.New(ep.Error())
	}
	return nil
}
