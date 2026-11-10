// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

// IdentityH and IdentityV are the predefined Identity-H and Identity-V
// CMaps, which map two-byte codes to the CID of the same value.  They
// must not be modified.
var (
	IdentityH = mustPredefined("Identity-H")
	IdentityV = mustPredefined("Identity-V")
)

func mustPredefined(name string) *File {
	c, err := Predefined(name)
	if err != nil {
		panic(err)
	}
	return c
}
