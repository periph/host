// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package host

import (
	// Make sure CPU and board drivers are registered.
	_ "adev73/x/host/v3/allwinner"
	_ "adev73/x/host/v3/am335x"
	_ "adev73/x/host/v3/bcm283x"
	_ "adev73/x/host/v3/beagle/bone"
	_ "adev73/x/host/v3/beagle/green"
	_ "adev73/x/host/v3/chip"
	_ "adev73/x/host/v3/odroidc1"

	// While this board is ARM64, it may run ARM 32 bits binaries so load it on
	// 32 bits builds too.
	_ "adev73/x/host/v3/pine64"
	_ "adev73/x/host/v3/rpi"
)
