// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/postscript/cid"
)

// TestMakeCodecInherited checks that a CMap which takes its code space from
// a parent CMap, as Identity-V does from Identity-H, decodes two-byte codes.
func TestMakeCodecInherited(t *testing.T) {
	if len(cmap.IdentityV.CodeSpaceRange) != 0 || cmap.IdentityV.Parent == nil {
		t.Fatal("Identity-V does not inherit its code space")
	}

	codec := makeCodec(cmap.IdentityV, nil)
	s := pdf.String{0x01, 0x02}
	code, k, valid := codec.Decode(s)
	if !valid || k != 2 || string(codec.AppendCode(nil, code)) != string(s) {
		t.Errorf("decoded (%d, %d, %t), want the two-byte code % x", code, k, valid, s)
	}
}

// TestMakeCodecCycle checks that a cycle of Parent links does not make
// makeCodec loop forever.
func TestMakeCodecCycle(t *testing.T) {
	a := &cmap.File{CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00, 0x00}, High: []byte{0xFF, 0xFF}}}}
	b := &cmap.File{Parent: a}
	a.Parent = b

	codec := makeCodec(a, nil)
	if _, k, valid := codec.Decode(pdf.String{0x01, 0x02}); !valid || k != 2 {
		t.Errorf("decoded (%d, %t), want a valid two-byte code", k, valid)
	}
}

// TestMakeCodecUseCMapStream checks that code space ranges are inherited
// through a usecmap chain of embedded CMap streams.
func TestMakeCodecUseCMapStream(t *testing.T) {
	ros := &cid.SystemInfo{Registry: "Test", Ordering: "Chain"}
	root := &cmap.File{
		Name:           "Test-Root",
		ROS:            ros,
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00, 0x00}, High: []byte{0xFF, 0xFF}}},
		CIDRanges:      []cmap.Range{{First: []byte{0x00, 0x00}, Last: []byte{0x00, 0xFF}, Value: 1}},
	}
	middle := &cmap.File{Name: "Test-Middle", ROS: ros, Parent: root}
	leaf := &cmap.File{Name: "Test-Leaf", ROS: ros, WMode: font.Vertical, Parent: middle}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(leaf)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := cmap.Extract(pdf.NewCursor(w), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Parent == nil || got.Parent.Parent == nil || got.Parent.Parent.IsPredefined() {
		t.Fatal("usecmap chain not read from the embedded streams")
	}
	if len(got.CodeSpaceRange) != 0 || len(got.Parent.CodeSpaceRange) != 0 {
		t.Fatal("unexpected code space ranges in the child CMaps")
	}

	codec := makeCodec(got, nil)
	s := pdf.String{0x00, 0x05}
	code, k, valid := codec.Decode(s)
	if !valid || k != 2 || string(codec.AppendCode(nil, code)) != string(s) {
		t.Errorf("decoded (%d, %d, %t), want the two-byte code % x", code, k, valid, s)
	}
	if c := got.LookupCID(s); c != 6 {
		t.Errorf("CID %d, want 6", c)
	}
}

// TestMakeCodecPrefix checks that makeCodec uses the longest initial part of
// the code space ranges which forms a valid codec.
func TestMakeCodecPrefix(t *testing.T) {
	one := charcode.Range{Low: []byte{0x00}, High: []byte{0x7F}}
	two := charcode.Range{Low: []byte{0x80, 0x00}, High: []byte{0xFF, 0xFF}}
	conflict := charcode.Range{Low: []byte{0x00, 0x00}, High: []byte{0x00, 0xFF}}

	for n := range 6 {
		csr := charcode.CodeSpaceRange{one, two}
		for range n {
			csr = append(csr, conflict)
		}
		codec := makeCodec(&cmap.File{CodeSpaceRange: csr}, nil)
		if !codec.CodeSpaceRange().Equivalent(charcode.CodeSpaceRange{one, two}) {
			t.Errorf("%d conflicts: code space %v", n, codec.CodeSpaceRange())
		}
	}

	codec := makeCodec(&cmap.File{CodeSpaceRange: charcode.CodeSpaceRange{one, conflict}}, nil)
	if !codec.CodeSpaceRange().Equivalent(charcode.CodeSpaceRange{one}) {
		t.Errorf("code space %v, want %v", codec.CodeSpaceRange(), one)
	}

	codec = makeCodec(&cmap.File{}, nil)
	if !codec.CodeSpaceRange().Equivalent(charcode.Simple) {
		t.Errorf("code space %v, want one-byte codes", codec.CodeSpaceRange())
	}
}

// TestMakeCodecScaling checks that the work makeCodec does for a list of
// code space ranges which conflict from the second range on grows nearly
// linearly with the length of the list.  Allocations stand in for work,
// since each attempt to build a codec allocates.
func TestMakeCodecScaling(t *testing.T) {
	csrOf := func(n int) charcode.CodeSpaceRange {
		csr := charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}}
		for range n - 1 {
			csr = append(csr, charcode.Range{Low: []byte{0x00, 0x00}, High: []byte{0xFF, 0xFF}})
		}
		return csr
	}
	allocs := func(n int) float64 {
		cm := &cmap.File{CodeSpaceRange: csrOf(n)}
		return testing.AllocsPerRun(5, func() { makeCodec(cm, nil) })
	}

	small, large := allocs(1000), allocs(16000)
	// 16 times as many ranges: a quadratic algorithm makes about 16 times as
	// many attempts, each over a list 16 times as long
	if large > 4*small {
		t.Errorf("%g allocations for 16000 ranges, %g for 1000", large, small)
	}
}

// TestMakeCodecFamiliesScaling checks the work for a list of ranges where
// each attempt to build a codec has to look at all byte positions: for every
// byte position d there are k four-byte ranges which pin byte d to an even
// value 2i and leave the other bytes free.  A final one-byte range conflicts
// with all of them.  Building the lookup tree without sharing sub-trees
// takes time exponential in the code length for this list; with sharing,
// long prefixes still exceed the construction budget.
func TestMakeCodecFamiliesScaling(t *testing.T) {
	csrOf := func(k int) charcode.CodeSpaceRange {
		var csr charcode.CodeSpaceRange
		for d := range 4 {
			for i := range k {
				lo := []byte{0x00, 0x00, 0x00, 0x00}
				hi := []byte{0xFF, 0xFF, 0xFF, 0xFF}
				lo[d], hi[d] = byte(2*i), byte(2*i)
				csr = append(csr, charcode.Range{Low: lo, High: hi})
			}
		}
		return append(csr, charcode.Range{Low: []byte{0x00}, High: []byte{0xFF}})
	}
	allocs := func(k int) float64 {
		cm := &cmap.File{CodeSpaceRange: csrOf(k)}
		return testing.AllocsPerRun(3, func() { makeCodec(cm, nil) })
	}

	small, large := allocs(32), allocs(64)
	t.Logf("%g allocations for k=32, %g for k=64", small, large)
	// twice as many ranges: the work per attempt is bounded by the
	// construction budget, which is linear in the number of ranges
	if large > 4*small {
		t.Errorf("%g allocations for k=64, %g for k=32", large, small)
	}

	codec := makeCodec(&cmap.File{CodeSpaceRange: csrOf(64)}, nil)
	if _, n, valid := codec.Decode([]byte{0x01, 0x02, 0x03, 0x04}); n != 4 || !valid {
		t.Errorf("decode: %d bytes, valid=%t", n, valid)
	}
}

// slabs returns 3k four-byte code space ranges whose lookup tree grows like
// a high power of k: for every byte position d < 3 there are k ranges which
// fix the byte at position d to 2i and the last byte to 3i+d.
func slabs(k int) charcode.CodeSpaceRange {
	var csr charcode.CodeSpaceRange
	for d := range 3 {
		for i := range k {
			lo := []byte{0x00, 0x00, 0x00, byte(3*i + d)}
			hi := []byte{0xFF, 0xFF, 0xFF, byte(3*i + d)}
			lo[d], hi[d] = byte(2*i), byte(2*i)
			csr = append(csr, charcode.Range{Low: lo, High: hi})
		}
	}
	return csr
}

// TestMakeCodecTooComplex checks that a code space whose lookup tree would
// be too large gives one-byte codes, with work linear in the number of
// ranges.
func TestMakeCodecTooComplex(t *testing.T) {
	if _, err := charcode.NewCodec(slabs(64)); err != charcode.ErrTooComplex {
		t.Fatalf("got error %v, want ErrTooComplex", err)
	}

	allocs := func(k int) float64 {
		cm := &cmap.File{CodeSpaceRange: slabs(k)}
		return testing.AllocsPerRun(3, func() {
			codec := makeCodec(cm, nil)
			if codec != charcode.SimpleCodec {
				t.Fatalf("%d ranges: code space %v", 3*k, codec.CodeSpaceRange())
			}
		})
	}
	small, large := allocs(32), allocs(64)
	t.Logf("%g allocations for 96 ranges, %g for 192", small, large)
	if large > 3*small {
		t.Errorf("%g allocations for 192 ranges, %g for 96", large, small)
	}
}

// TestMakeCodecTooComplexPrefix checks the search for a valid prefix of an
// invalid list of code space ranges, where the lookup trees of long prefixes
// would be too large.
func TestMakeCodecTooComplexPrefix(t *testing.T) {
	csrOf := func(k int) charcode.CodeSpaceRange {
		return append(slabs(k), charcode.Range{Low: []byte{0x00}, High: []byte{0xFF}})
	}

	for _, k := range []int{32, 64} {
		csr := csrOf(k)
		codec := makeCodec(&cmap.File{CodeSpaceRange: csr}, nil)
		got := codec.CodeSpaceRange()
		if len(got) == 0 || len(got) >= len(csr) {
			t.Fatalf("%d ranges: code space %v", len(csr), got)
		}
		if !got.Equivalent(csr[:len(got)]) {
			t.Errorf("%d ranges: code space is not a prefix of the list", len(csr))
		}
		again := makeCodec(&cmap.File{CodeSpaceRange: csr}, nil)
		if !again.CodeSpaceRange().Equivalent(got) {
			t.Errorf("%d ranges: result is not deterministic", len(csr))
		}
	}

	allocs := func(k int) float64 {
		cm := &cmap.File{CodeSpaceRange: csrOf(k)}
		return testing.AllocsPerRun(3, func() { makeCodec(cm, nil) })
	}
	small, large := allocs(32), allocs(64)
	t.Logf("%g allocations for k=32, %g for k=64", small, large)
	// O(log n) attempts, each with work bounded linearly in n
	if large > 3*small {
		t.Errorf("%g allocations for k=64, %g for k=32", large, small)
	}
}

// TestMakeCodecJohab checks the codes <D831>..<D840> of the predefined
// KSC-Johab CMaps.  These are valid codes, which an earlier version of the
// lookup tree rejected.
func TestMakeCodecJohab(t *testing.T) {
	for _, name := range []string{"KSC-Johab-H", "KSC-Johab-V"} {
		cm, err := cmap.Predefined(name)
		if err != nil {
			t.Fatal(err)
		}
		codec := makeCodec(cm, nil)
		for _, s := range []pdf.String{{0xD8, 0x31}, {0xD8, 0x40}, {0xD8, 0x41}} {
			code, k, valid := codec.Decode(s)
			if !valid || k != 2 || string(codec.AppendCode(nil, code)) != string(s) {
				t.Errorf("%s: % x decoded as (%x, %d, %t)", name, s, code, k, valid)
			}
		}
	}
}
