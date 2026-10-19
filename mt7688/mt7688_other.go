// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Disable MT7688 processor detection on unsupported architectures.

//go:build !mipsle
// +build !mipsle

package mt7688

const isMIPS = false
