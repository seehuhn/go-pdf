// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package opaque represents PDF values which the library does not interpret.
// [Object] holds an arbitrary PDF value and implements [pdf.Embedder];
// [Stream] holds a PDF stream and is written with [Stream.WriteAt].
package opaque
