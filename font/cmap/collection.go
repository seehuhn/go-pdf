// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
)

// standardOrderings lists the orderings of the standardised Adobe character
// collections.  Adobe-Japan2 and Adobe-Korea1 are deprecated in PDF 2.0, but
// remain valid in files written for earlier versions.  Adobe-KR was added in
// PDF 2.0, see [CheckCollectionVersion].
var standardOrderings = map[string]bool{
	"CNS1":   true,
	"GB1":    true,
	"Japan1": true,
	"Japan2": true,
	"KR":     true,
	"Korea1": true,
}

// IsStandardCollection reports whether ros is one of the standardised Adobe
// character collections: Adobe-CNS1, Adobe-GB1, Adobe-Japan1, Adobe-Japan2,
// Adobe-KR or Adobe-Korea1.  The supplement is ignored.
//
// The CIDs of these collections identify glyphs independently of any
// particular font program.  A CIDFont without an embedded font program must
// use one of them, since only then can a substitute font select the intended
// glyphs.
func IsStandardCollection(ros *cid.SystemInfo) bool {
	return ros != nil && ros.Registry == "Adobe" && standardOrderings[ros.Ordering]
}

// CheckCollectionVersion checks that the PDF file being written supports ros
// as a standard character collection, as needed for a CIDFont without an
// embedded font program.  Adobe-KR requires PDF 2.0; the other collections
// accepted by [IsStandardCollection] have no version requirement beyond that
// of composite fonts.
func CheckCollectionVersion(w *pdf.Writer, ros *cid.SystemInfo) error {
	if ros != nil && ros.Registry == "Adobe" && ros.Ordering == "KR" {
		return pdf.CheckVersion(w, "non-embedded Adobe-KR font", pdf.V2_0)
	}
	return nil
}
