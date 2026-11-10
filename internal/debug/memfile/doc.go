// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package memfile provides an in-memory file implementation that satisfies the
// [io.ReadWriteSeeker] interface.  It can be used for unit tests which
// need to generate temporary PDF files.
package memfile
