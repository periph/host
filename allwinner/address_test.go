// Copyright 2024 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package allwinner

import (
	"os"
	"path"
	"testing"
)

func createDirs(t *testing.T, root string, dirs ...string) string {
	for _, dir := range dirs {
		if err := os.MkdirAll(path.Join(root, dir), os.ModePerm); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func createFiles(t *testing.T, root string, paths ...string) string {
	for _, path_ := range paths {
		if file, err := os.Create(path.Join(root, path_)); err != nil {
			t.Fatal(err)
		} else {
			file.Close()
		}
	}
	return root
}

func createSymLink(t *testing.T, root string, source string, destination string) {

	if err := os.Symlink(path.Join(root, source), path.Join(root, destination)); err != nil {
		t.Fatal(err)
	}
}

func TestGetDefaultBaseAddress_default(t *testing.T) {
	want := uint64(0x01C20800)
	if address := getDefaultBaseAddress("/dev/null"); address != want {
		t.Errorf("Expected %d received %d", want, address)
	}
}

func TestGetDefaultBaseAddress(t *testing.T) {
	root := t.TempDir()
	createDirs(t,
		root,
		"sun50i-pinctrl/bin",
		"foo",
	)
	createFiles(t, root, "foo/300b000.pinctrl")
	createSymLink(t, root, "foo/300b000.pinctrl", "sun50i-pinctrl/driver")
	want := uint64(0x300b000)
	if address := getDefaultBaseAddress(root); address != want {
		t.Errorf("Expected %d received %d", want, address)
	}
}

func TestGetBaseAddressForH6CPU(t *testing.T) {
	root := t.TempDir()
	createDirs(t,
		root,
		"sun50i-h6-pinctrl/bin",
		"sun50i-h6-pinctrl/uevent",
		"sun50i-h6-pinctrl/ubind",
		"sun50i-h616-pinctrl/ubind",
		"sun50i-h616-pinctrl/uevent",
		"sun50i-h616-pinctrl/bin",
	)
	createFiles(t, root, "sun50i-h616-pinctrl/300b000.pinctrl")
	if val, err := getBaseAddressForH6CPU(root); err != nil {
		t.Error(err)
	} else if val != uint64(0x300b000) {
		t.Fail()
	}
}

func TestGetBaseAddressForH6CPU_default(t *testing.T) {
	root := t.TempDir()
	createDirs(t,
		root,
		"sun50i-h6-pinctrl/bin",
		"sun50i-h6-pinctrl/uevent",
		"sun50i-h6-pinctrl/ubind",
	)
	if _, err := getBaseAddressForH6CPU(root); err == nil {
		t.Fail()
	}
}
