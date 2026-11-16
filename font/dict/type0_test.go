// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/glyphdata"
)

// TestCIDFontType0ExternalCollection checks that a non-embedded font must
// use a standard character collection.
func TestCIDFontType0ExternalCollection(t *testing.T) {
	japan1, err := cmap.Predefined("UniJIS-UTF16-H")
	if err != nil {
		t.Fatal(err)
	}
	custom := func(registry, ordering string) *cmap.File {
		return &cmap.File{
			Name:           "Test-cmap",
			ROS:            &cid.SystemInfo{Registry: registry, Ordering: ordering},
			CodeSpaceRange: charcode.Simple,
			CIDSingles:     []cmap.Single{{Code: []byte{' '}, Value: 1}},
		}
	}
	for _, tc := range []struct {
		label    string
		cmap     *cmap.File
		embedded bool
		ok       bool
	}{
		{"Identity-H", cmap.IdentityH, true, true},
		{"Identity-V", cmap.IdentityV, true, true},
		{"Identity-H", cmap.IdentityH, false, false},
		{"Identity-V", cmap.IdentityV, false, false},
		{"copy of Identity-H", cmap.IdentityH.Clone(), false, false},
		{"UniJIS-UTF16-H", japan1, false, true},
		{"custom Adobe-Identity", custom("Adobe", "Identity"), false, false},
		{"custom Quire-Test", custom("Quire", "Test"), true, true},
		{"custom Quire-Test", custom("Quire", "Test"), false, false},
		{"custom Adobe-Japan1", custom("Adobe", "Japan1"), false, true},
	} {
		d := &CIDFontType0{
			PostScriptName: "Test",
			Descriptor:     &font.Descriptor{FontName: "Test"},
			ROS:            tc.cmap.ROS,
			CMap:           tc.cmap,
		}
		if tc.embedded {
			d.FontFile = &glyphdata.Stream{Type: glyphdata.CFF}
		}
		err := d.validate()
		if ok := err == nil; ok != tc.ok {
			t.Errorf("%s, embedded=%t: got error %v", tc.label, tc.embedded, err)
		}
	}
}

// TestCIDFontType0AdobeKRVersion checks that a non-embedded font can use the
// Adobe-KR character collection only in PDF 2.0 files.
func TestCIDFontType0AdobeKRVersion(t *testing.T) {
	cm := &cmap.File{
		Name:           "Test-cmap",
		ROS:            &cid.SystemInfo{Registry: "Adobe", Ordering: "KR", Supplement: 9},
		CodeSpaceRange: charcode.Simple,
		CIDSingles:     []cmap.Single{{Code: []byte{' '}, Value: 1}},
	}
	for _, tc := range []struct {
		v  pdf.Version
		ok bool
	}{
		{pdf.V1_7, false},
		{pdf.V2_0, true},
	} {
		w, _ := memfile.NewPDFWriter(t, tc.v, nil)
		rm := pdf.NewResourceManager(w)
		d := &CIDFontType0{
			PostScriptName: "Test",
			Descriptor:     &font.Descriptor{FontName: "Test"},
			ROS:            cm.ROS,
			CMap:           cm,
		}
		_, err := rm.Embed(d)
		if tc.ok && err != nil {
			t.Errorf("PDF %s: unexpected error: %v", tc.v, err)
		} else if !tc.ok && !pdf.IsWrongVersion(err) {
			t.Errorf("PDF %s: got %v, want version error", tc.v, err)
		}
	}
}
