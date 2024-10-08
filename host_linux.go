// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package host

import (
	// Make sure required drivers are registered.
	_ "periph.io/x/host/v3/gpioioctl"
	_ "periph.io/x/host/v3/netlink"
	_ "periph.io/x/host/v3/sysfs"
)
