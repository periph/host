// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package allwinner

import (
	"errors"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// getBaseAddress queries the virtual file system to retrieve the base address
// of the GPIO registers for GPIO pins in groups PA to PI.
//
// Defaults to 0x01C20800 as per datasheet if it could not query the file
// system.
func getBaseAddress() (uint64, error) {
	driverDir := "/sys/bus/platform/drivers"
	if IsH6() {
		return getBaseAddressForH6CPU(driverDir)
	}
	return getDefaultBaseAddress(driverDir), nil
}

func getDefaultBaseAddress(driverDir string) uint64 {
	base := uint64(0x01C20800)
	link, err := os.Readlink(path.Join(driverDir, "sun50i-pinctrl/driver"))
	if err != nil {
		return base
	}
	parts := strings.SplitN(path.Base(link), ".", 2)
	if len(parts) != 2 {
		return base
	}
	base2, err := strconv.ParseUint(parts[0], 16, 64)
	if err != nil {
		return base
	}
	return base2
}

func getBaseAddressForH6CPU(driverDir string) (uint64, error) {
	items, err := os.ReadDir(driverDir)
	if err != nil {
		return 0, err
	}
	return getBaseAddressFromDirItemsForH6CPU(driverDir, items)
}

func getBaseAddressFromDirItemsForH6CPU(root string, items []os.DirEntry) (uint64, error) {
	for _, item := range items {
		if ret, ok := getBaseAddressFromDirItemForH6CPU(root, item); ok {
			return ret, nil
		}

	}
	return 0, errors.New("file with base address not found")
}

func getBaseAddressFromDirItemForH6CPU(root string, item os.DirEntry) (uint64, bool) {
	if !item.IsDir() {
		return 0, false
	}

	if matched, _ := regexp.MatchString(`^sun50i-h6\d*-pinctrl$`, item.Name()); !matched {
		return 0, false
	}

	return extractBaseAddressFromDriverDirForH6CPU(path.Join(root, item.Name()))
}

func extractBaseAddressFromDriverDirForH6CPU(dir string) (uint64, bool) {
	if fileInfo, err := os.Stat(dir); err != nil || !fileInfo.IsDir() {
		return 0, false
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return 0, false
	}
	for _, item := range items {
		if address, ok := extractBaseAddress(item); ok {
			return address, ok
		}
	}
	return 0, false
}

func extractBaseAddress(item os.DirEntry) (uint64, bool) {
	if item.IsDir() {
		return 0, false
	}
	if !strings.HasSuffix(item.Name(), ".pinctrl") {
		return 0, false
	}
	prefix := item.Name()[:len(item.Name())-len(".pinctrl")]
	if address, err := strconv.ParseUint(prefix, 16, 64); err != nil {
		return 0, false
	} else {
		return address, true
	}

}
