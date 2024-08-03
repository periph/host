package allwinner

import (
	"os"
	"path"
	"testing"
)

func createDirs(root string, dirs ...string) string {
	for _, dir := range dirs {
		if err := os.MkdirAll(path.Join(root, dir), os.ModePerm); err != nil {
			panic(err)
		}
	}
	return root
}

func createFiles(root string, paths ...string) string {
	for _, path_ := range paths {
		if file, err := os.Create(path.Join(root, path_)); err != nil {
			panic(err)
		} else {
			file.Close()
		}
	}
	return root
}

func createSymLink(root string, source string, destination string) {

	if err := os.Symlink(path.Join(root, source), path.Join(root, destination)); err != nil {
		panic(err)
	}
}

func Test_getDefaultBaseAddress_default(t *testing.T) {
	expt := uint64(0x01C20800)
	if address := getDefaultBaseAddress("/dev/null"); address != expt {
		t.Errorf("Expected %d received %d", expt, address)
	}
}

func Test_getDefaultBaseAddress(t *testing.T) {
	root := t.TempDir()
	createDirs(root,
		"sun50i-pinctrl/bin",
		"foo",
	)
	createFiles(root, "foo/300b000.pinctrl")
	createSymLink(root, "foo/300b000.pinctrl", "sun50i-pinctrl/driver")
	expt := uint64(0x300b000)
	if address := getDefaultBaseAddress(root); address != expt {
		t.Errorf("Expected %d received %d", expt, address)
	}
}

func Test_getBaseAddressForH6CPU(t *testing.T) {
	root := t.TempDir()
	createDirs(root,
		"sun50i-h6-pinctrl/bin",
		"sun50i-h6-pinctrl/uevent",
		"sun50i-h6-pinctrl/ubind",
		"sun50i-h616-pinctrl/ubind",
		"sun50i-h616-pinctrl/uevent",
		"sun50i-h616-pinctrl/bin",
	)
	createFiles(root, "sun50i-h616-pinctrl/300b000.pinctrl")
	if val, err := getBaseAddressForH6CPU(root); err != nil {
		t.Error(err)
	} else if val != uint64(0x300b000) {
		t.Fail()
	}
}

func Test_getBaseAddressForH6CPU_default(t *testing.T) {
	root := t.TempDir()
	createDirs(root,
		"sun50i-h6-pinctrl/bin",
		"sun50i-h6-pinctrl/uevent",
		"sun50i-h6-pinctrl/ubind",
	)
	if _, err := getBaseAddressForH6CPU(root); err == nil {
		t.Fail()
	}
}
