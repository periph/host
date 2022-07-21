// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ftdi_test

import (
	"fmt"
	"log"

	"adev73/x/host/v3"
	"adev73/x/host/v3/ftdi"
)

func Example() {
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}
	for _, d := range ftdi.All() {
		fmt.Printf("%s\n", d)
	}
}
