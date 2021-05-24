// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ftdi

import (
	"testing"

	"periph.io/x/d2xx"
	"periph.io/x/d2xx/d2xxtest"
)

func TestDriver(t *testing.T) {
	defer reset(t)
	drv.numDevices = func() (int, error) {
		return 1, nil
	}
	drv.d2xxOpen = func(i int) (d2xx.Handle, d2xx.Err) {
		if i != 0 {
			t.Fatalf("unexpected index %d", i)
		}
		d := &d2xxtest.Fake{
			DevType: uint32(DevTypeFT232R),
			Vid:     0x0403,
			Pid:     0x6014,
			Data:    [][]byte{{}, {0}},
		}
		return d, 0
	}
	if b, err := drv.Init(); !b || err != nil {
		t.Fatalf("Init() = %t, %v", b, err)
	}
}

func reset(t *testing.T) {
	drv.reset()
}

func init() {
	reset(nil)
}
