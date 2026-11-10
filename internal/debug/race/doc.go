// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package race reports whether the race detector is enabled.
//
// A test which makes a claim only the race detector can refute, for example
// that readers may use a data structure while it is being written, uses this
// to skip itself in an ordinary test run.  The test is still compiled and
// vetted there, so it cannot quietly rot between race runs.
//
// The race detector is chosen when the test binary is linked, so a test
// cannot turn it on for itself: run "go test -race" to exercise these tests.
package race
