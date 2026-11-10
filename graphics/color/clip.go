// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package color

// ClipComponent returns v moved to the nearest value in [lo, hi].
//
// A colour component outside the range its colour space allows is adjusted to
// the nearest valid value, without error indication, and the same rule applies
// to the numeric parameters of the graphics state.  This function is the
// single definition of that adjustment; readers use it to repair values taken
// from a file, and writers use it to decide whether a value they are given
// would survive a round trip unchanged.
//
// The first comparison is negated so that a NaN, which is unordered against
// every bound, yields lo.  The result is therefore finite whenever lo and hi
// are.
func ClipComponent(v, lo, hi float64) float64 {
	if !(v >= lo) {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ClipComponents clips the components of a colour into the ranges the colour
// space allows, as reported by [Space.ComponentRange].  The values are
// modified in place and returned.  Values beyond the dimensionality of the
// space are left unchanged.
func ClipComponents(space Space, values []float64) []float64 {
	for i := range min(len(values), space.Channels()) {
		lo, hi := space.ComponentRange(i)
		values[i] = ClipComponent(values[i], lo, hi)
	}
	return values
}

// clip01 clips a component of a device colour space to [0, 1].  Callers which
// hold their components as separate scalars use this rather than
// [ClipComponents], which would need a slice built for it.
func clip01(v float64) float64 {
	return ClipComponent(v, 0, 1)
}
