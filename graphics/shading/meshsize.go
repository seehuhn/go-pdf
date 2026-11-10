// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package shading

import "unsafe"

// In-memory sizes of the per-element mesh structures, excluding the color
// backing arrays.  They give the per-element cost charged against the
// membudget.Budget in [parseType4Vertices] and friends.  unsafe is confined to
// these declarations so the use is easy to audit.
const (
	type4VertexSize = int64(unsafe.Sizeof(Type4Vertex{}))
	type5VertexSize = int64(unsafe.Sizeof(Type5Vertex{}))
	type6PatchSize  = int64(unsafe.Sizeof(Type6Patch{}))
	type7PatchSize  = int64(unsafe.Sizeof(Type7Patch{}))
)
