// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
