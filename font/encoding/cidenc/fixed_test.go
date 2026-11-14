// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidenc

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/cid"
)

// TestFixedChildWins checks that a code the CMap remaps encodes the child's
// CID and decodes back to it, and that the parent's CID for that code is
// left without a code rather than emitting one which decodes differently.
func TestFixedChildWins(t *testing.T) {
	f, err := cmap.Predefined("UniJIS-UCS2-HW-H")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := NewFromCMap(f, f.ROS, 500, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}

	code := []byte{0x00, 'A'}
	child := f.LookupCID(code)
	parent := f.Parent.LookupCID(code)
	if child == parent {
		t.Fatal("test CMap does not remap ASCII")
	}

	c, err := enc.Encode(child, "A", 500, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	var s pdf.String
	s = enc.Codec().AppendCode(s, c)
	for got := range enc.Codes(s) {
		if got.CID != child {
			t.Errorf("code decodes to CID %d, want %d", got.CID, child)
		}
	}

	if _, err := enc.Encode(parent, "A", 500, VMetrics{}); err == nil {
		t.Errorf("shadowed CID %d was encoded", parent)
	}
}

// TestFixedSmallestCode checks that a CID which several codes map to is
// always written as the smallest of them.
func TestFixedSmallestCode(t *testing.T) {
	f, err := cmap.Predefined("UniJIS-UCS2-H")
	if err != nil {
		t.Fatal(err)
	}
	codec, err := f.Codec()
	if err != nil {
		t.Fatal(err)
	}

	// find a CID with more than one code
	codes := make(map[cid.CID][]charcode.Code)
	for code, c := range f.All(codec) {
		codes[c] = append(codes[c], code)
	}
	var shared cid.CID
	var want charcode.Code
	for c, cc := range codes {
		if len(cc) > 1 {
			shared, want = c, slices.Min(cc)
			break
		}
	}
	if len(codes[shared]) < 2 {
		t.Fatal("test CMap has no CID with several codes")
	}

	// several encoders are built, since map order varies between runs
	for range 10 {
		enc, err := NewFromCMap(f, f.ROS, 500, VMetrics{})
		if err != nil {
			t.Fatal(err)
		}
		got, err := enc.Encode(shared, "x", 500, VMetrics{})
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("CID %d written as code %d, want %d", shared, got, want)
		}
	}
}

// TestFixedVMetrics checks that the vertical metrics given to Encode are
// stored per CID, reported by MappedCodes, and turned into the vertical
// fields of font.Code by Codes in vertical writing mode only.  CID 0 carries
// the metrics given to the constructor, and so does a CID never encoded.
func TestFixedVMetrics(t *testing.T) {
	cid0V := VMetrics{OffsY: 900, DeltaY: -1100}
	vm := VMetrics{OffsY: 750, DeltaY: -800}
	for _, tc := range []struct {
		cmap *cmap.File
		want []font.Code
	}{
		{cmap.IdentityH, []font.Code{
			{CID: 7, Width: 0.6, Text: "x"},
			{CID: 0, Width: 0.5},
			{CID: 9, Width: 0.5},
		}},
		{cmap.IdentityV, []font.Code{
			{CID: 7, Width: 0.6, Text: "x", VerticalAdvance: -0.8, VerticalOriginX: 0.3, VerticalOriginY: 0.75},
			{CID: 0, Width: 0.5, VerticalAdvance: -1.1, VerticalOriginX: 0.25, VerticalOriginY: 0.9},
			{CID: 9, Width: 0.5, VerticalAdvance: -1.1, VerticalOriginX: 0.25, VerticalOriginY: 0.9},
		}},
	} {
		enc, err := NewFromCMap(tc.cmap, tc.cmap.ROS, 500, cid0V)
		if err != nil {
			t.Fatal(err)
		}
		code, err := enc.Encode(7, "x", 600, vm)
		if err != nil {
			t.Fatal(err)
		}

		for c, info := range enc.MappedCodes() {
			if c == code && info.VMetrics != vm {
				t.Errorf("MappedCodes: got %v, want %v", info.VMetrics, vm)
			}
		}

		s := enc.Codec().AppendCode(nil, code)
		s = enc.Codec().AppendCode(s, 0) // CID 0
		s = append(s, 0x00, 0x09)        // CID 9, not encoded
		got := slices.Collect(enc.Codes(pdf.String(s)))
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("%s: Codes (-want +got):\n%s", tc.cmap.Name, diff)
		}
		if w := enc.Width(0x0900); w != 500 {
			t.Errorf("%s: width of an unencoded CID is %g, want 500", tc.cmap.Name, w)
		}

		// CID 0 already has metrics, so different ones are a conflict in
		// vertical writing mode
		_, err = enc.Encode(0, "", 500, VMetrics{})
		if conflict := err != nil; conflict != (tc.cmap.WMode == font.Vertical) {
			t.Errorf("%s: Encode with other CID 0 metrics gave %v", tc.cmap.Name, err)
		}
	}
}

// TestFixedToUnicodeInherited checks that the ToUnicode CMap of an encoder
// whose CMap takes its code space from a parent, as Identity-V does, covers
// the codes in use.
func TestFixedToUnicodeInherited(t *testing.T) {
	enc, err := NewFromCMap(cmap.IdentityV, cmap.IdentityV.ROS, 500, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	code, err := enc.Encode(0x0102, "A", 600, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}

	tu := enc.ToUnicode()
	if tu == nil {
		t.Fatal("no ToUnicode CMap")
	}
	codec, err := charcode.NewCodec(tu.CodeSpaceRange)
	if err != nil {
		t.Fatal(err)
	}
	got := maps.Collect(tu.All(codec))
	if want := map[charcode.Code]string{code: "A"}; !maps.Equal(got, want) {
		t.Errorf("ToUnicode maps %v, want %v", got, want)
	}
}

// TestFixedToUnicodeCodeSpace checks the code space of the ToUnicode CMap:
// for a CMap with a code space of its own it is that code space, ranges in
// their original order, and for a CMap which inherits its code space through
// usecmap it is the inherited one.
func TestFixedToUnicodeCodeSpace(t *testing.T) {
	for _, name := range []string{"90ms-RKSJ-H", "90ms-RKSJ-V", "UniJIS-UTF16-H", "UniJIS-UTF16-V", "B5pc-H"} {
		t.Run(name, func(t *testing.T) {
			f, err := cmap.Predefined(name)
			if err != nil {
				t.Fatal(err)
			}
			enc, err := NewFromCMap(f, f.ROS, 500, VMetrics{})
			if err != nil {
				t.Fatal(err)
			}

			// encode a few CIDs, with a text the collection does not imply
			want := make(map[charcode.Code]string)
			for _, c := range f.All(enc.Codec()) {
				if len(want) >= 3 {
					break
				}
				if c == 0 {
					continue
				}
				got, err := enc.Encode(c, "\ue000", 500, VMetrics{})
				if err != nil {
					t.Fatal(err)
				}
				// the CID may have a smaller code than the one reported
				want[got] = "\ue000"
			}

			tu := enc.ToUnicode()
			if tu == nil {
				t.Fatal("no ToUnicode CMap")
			}
			if f.Parent == nil {
				wantTU, err := cmap.NewToUnicodeFile(f.CodeSpaceRange, want)
				if err != nil {
					t.Fatal(err)
				}
				if d := cmp.Diff(wantTU, tu); d != "" {
					t.Errorf("ToUnicode CMap (-want +got):\n%s", d)
				}
				return
			}

			if len(f.CodeSpaceRange) != 0 {
				t.Fatalf("test assumes that %s has no code space of its own", name)
			}
			if !tu.CodeSpaceRange.Equivalent(enc.Codec().CodeSpaceRange()) {
				t.Errorf("code space %v, want that of %s", tu.CodeSpaceRange, f.Parent.Name)
			}
			codec, err := charcode.NewCodec(tu.CodeSpaceRange)
			if err != nil {
				t.Fatal(err)
			}
			if got := maps.Collect(tu.All(codec)); !maps.Equal(got, want) {
				t.Errorf("ToUnicode maps %v, want %v", got, want)
			}
		})
	}
}

// TestCID0Metrics checks that both encoders report the metrics of CID 0
// they were created with.
func TestCID0Metrics(t *testing.T) {
	v := VMetrics{OffsY: 880, DeltaY: -1000}
	fixedEnc, err := NewFromCMap(cmap.IdentityV, nil, 750, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, enc := range []CIDEncoder{fixedEnc, NewCompositeUtf8(nil, 750, v, font.Vertical)} {
		if w, gotV := enc.CID0Metrics(); w != 750 || gotV != v {
			t.Errorf("%T: got %g %v, want 750 %v", enc, w, gotV, v)
		}
	}
}

// TestFixedGetCodeUnmapped checks that GetCode finds no code for CID 0 when
// the CMap gives CID 0 no code, even though the encoder knows its metrics.
func TestFixedGetCodeUnmapped(t *testing.T) {
	f := &cmap.File{
		ROS:            &cid.SystemInfo{Registry: "Test", Ordering: "Private"},
		CodeSpaceRange: charcode.Simple,
		CIDRanges:      []cmap.Range{{First: []byte{0x00}, Last: []byte{0x0F}, Value: 104}},
	}
	enc, err := NewFromCMap(f, f.ROS, 500, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := enc.GetCode(0, ""); ok {
		t.Errorf("CID 0 has code %d", c)
	}
	if _, err := enc.Encode(0, "", 500, VMetrics{}); err == nil {
		t.Error("CID 0 was encoded")
	}
}

// tooComplexCodeSpace is accepted by [charcode.NewCodec], but its overlaps
// cannot be removed within the work budget of [charcode.CodeSpaceRange.Disjoint].
var tooComplexCodeSpace = charcode.CodeSpaceRange{
	{Low: []byte{0x81, 0x7f, 0x7d, 0x34}, High: []byte{0xda, 0xfe, 0x8b, 0xff}},
	{Low: []byte{0x80, 0x80, 0xfe, 0x6e}, High: []byte{0xfe, 0xda, 0xff, 0xdc}},
	{Low: []byte{0x7f, 0xa4, 0x80, 0xe9}, High: []byte{0xdc, 0xa6, 0x80, 0xea}},
	{Low: []byte{0x01, 0xa0, 0x01, 0x81}, High: []byte{0xb8, 0xd3, 0xad, 0xe9}},
	{Low: []byte{0x00, 0x32, 0x31, 0x7f}, High: []byte{0xe6, 0xf9, 0xfe, 0xfe}},
	{Low: []byte{0x05, 0x01, 0x08, 0x80}, High: []byte{0xcd, 0x92, 0xa2, 0xd8}},
	{Low: []byte{0x02, 0x80, 0x24, 0x01}, High: []byte{0xde, 0xfb, 0xfe, 0xc1}},
}

// TestFixedCodeSpaceTooComplex checks that a CMap whose code space cannot be
// written without overlaps is rejected, rather than giving a font without
// ToUnicode CMap.
func TestFixedCodeSpaceTooComplex(t *testing.T) {
	if _, err := charcode.NewCodec(tooComplexCodeSpace); err != nil {
		t.Fatal(err)
	}
	if _, err := tooComplexCodeSpace.Disjoint(); !errors.Is(err, charcode.ErrTooComplex) {
		t.Fatalf("Disjoint: got %v, want ErrTooComplex", err)
	}

	m := &cmap.File{
		Name:           "Test-Complex",
		ROS:            &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity"},
		CodeSpaceRange: tooComplexCodeSpace,
	}
	if _, err := NewFromCMap(m, m.ROS, 500, VMetrics{}); !errors.Is(err, charcode.ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
}

// TestFixedToUnicodeOverlap checks that overlapping code space ranges in a
// CMap and its parent give a ToUnicode CMap without overlaps.
func TestFixedToUnicodeOverlap(t *testing.T) {
	child := &cmap.File{
		Name:           "Test-Child",
		ROS:            cmap.IdentityH.ROS,
		Parent:         cmap.IdentityH,
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00, 0x00}, High: []byte{0x80, 0xFF}}},
	}
	enc, err := NewFromCMap(child, child.ROS, 500, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	code, err := enc.Encode(0x0102, "", 600, VMetrics{})
	if err != nil {
		t.Fatal(err)
	}

	tu := enc.ToUnicode()
	if tu == nil {
		t.Fatal("no ToUnicode CMap")
	}
	csr := tu.CodeSpaceRange
	for i := range csr {
		for j := range i {
			if len(csr[i].Low) == len(csr[j].Low) && csr[i].Low[0] <= csr[j].High[0] && csr[j].Low[0] <= csr[i].High[0] &&
				csr[i].Low[1] <= csr[j].High[1] && csr[j].Low[1] <= csr[i].High[1] {
				t.Errorf("ranges %v and %v overlap", csr[j], csr[i])
			}
		}
	}
	if !csr.Equivalent(enc.Codec().CodeSpaceRange()) {
		t.Errorf("code space %v differs from that of the encoder", csr)
	}
	codec, err := charcode.NewCodec(csr)
	if err != nil {
		t.Fatal(err)
	}
	got := maps.Collect(tu.All(codec))
	if want := map[charcode.Code]string{code: ""}; !maps.Equal(got, want) {
		t.Errorf("ToUnicode maps %v, want %v", got, want)
	}
}
