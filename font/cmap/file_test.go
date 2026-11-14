// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

var _ pdf.Embedder = (*File)(nil)

// The following variables contain test CMap data used in the tests below.
var (
	// testCMapParent is a CMap file which defines a horizontal CMap.
	// This is used as the parent CMap of testCMapChild.
	testCMapParent = []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap

/CMapName /TestH def
/CMapType 1 def
/WMode 0 def

/CIDSystemInfo 3 dict dup begin
  /Registry (Test) def
  /Ordering (Simple) def
  /Supplement 0 def
end def

1 begincodespacerange
<00> <FF>
endcodespacerange

1 begincidchar
<20> 1
endcidchar

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`)

	// testCMapChild is a CMap file which defines a vertical CMap.
	// This uses testCMapParent as the parent CMap, to test whether
	// extracting a chain of CMaps works correctly.
	testCMapChild = []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap

/CMapName /TestV def
/CMapType 1 def
/WMode 1 def

/TestH usecmap

/CIDSystemInfo 3 dict dup begin
  /Registry (Test) def
  /Ordering (Simple) def
  /Supplement 0 def
end def

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`)

	// testCMapFull is a CMap file which uses all supported CMap features.
	// This is used to get complete coverage of the CMap extraction code.
	testCMapFull = []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap

/CMapName /Test1 def
/CMapType 1 def
/WMode 1 def

/Test2 usecmap

/CIDSystemInfo 3 dict dup begin
  /Registry (Seehuhn) def
  /Ordering (Test) def
  /Supplement 42 def
end def

1 begincodespacerange
<20> <7F>
endcodespacerange

1 begincidchar
<20> 1
endcidchar

1 beginnotdefchar
<21> 2
endnotdefchar

1 begincidrange
<22> <24> 3
endcidrange

1 beginnotdefrange
<25> <27> 6
endnotdefrange

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`)

	// testInfoFull is an InfoNew struct which uses all fields of the struct.
	// This is used to test the template used to format a CMap file.
	testInfoFull = &File{
		Name: "Test",
		ROS: &cid.SystemInfo{
			Registry:   "Test",
			Ordering:   "Random",
			Supplement: 3,
		},
		WMode: font.Horizontal,
		Parent: &File{
			Name: "Other",
		},
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00}, High: []byte{0xFE}},
			{Low: []byte{0xFF, 0x00}, High: []byte{0xFF, 0xFF}},
		},
		CIDSingles: []Single{
			{Code: []byte{0x20}, Value: 2},
			{Code: []byte{0x22}, Value: 3},
		},
		CIDRanges: []Range{
			{First: []byte{0xFF, 0x20}, Last: []byte{0xFF, 0xFF}, Value: 5},
		},
		NotdefSingles: []Single{
			{Code: []byte{0x21}, Value: 1},
		},
		NotdefRanges: []Range{
			{First: []byte{0x00}, Last: []byte{0x1F}, Value: 0},
			{First: []byte{0xFF, 0x00}, Last: []byte{0xFF, 0x1F}, Value: 4},
		},
	}

	// The following two InfoNew structs are used to test embedding a chain of
	// CMaps.
	testROS = &cid.SystemInfo{
		Registry: "Seehuhn",
		Ordering: "Test",
	}
	testInfoParent = &File{
		Name:  "Test1",
		ROS:   testROS,
		WMode: font.Vertical,
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00, 0x00}, High: []byte{0x00, 0xFF}},
		},
		CIDSingles: []Single{
			{Code: []byte{0x20}, Value: 1},
		},
		CIDRanges: []Range{
			{First: []byte{0x21}, Last: []byte{0x23}, Value: 2},
		},
		NotdefSingles: []Single{
			{Code: []byte{0x24}, Value: 5},
		},
		NotdefRanges: []Range{
			{First: []byte{0x25}, Last: []byte{0x27}, Value: 6},
		},
	}
	testInfoChild = &File{
		Name:   "Test2",
		ROS:    testROS,
		WMode:  font.Vertical,
		Parent: testInfoParent,
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00, 0x00}, High: []byte{0x00, 0xFF}},
		},
		CIDSingles: []Single{
			{Code: []byte{0x28}, Value: 9},
		},
		CIDRanges: []Range{
			{First: []byte{0x29}, Last: []byte{0x2B}, Value: 10},
		},
		NotdefSingles: []Single{
			{Code: []byte{0x2C}, Value: 13},
		},
		NotdefRanges: []Range{
			{First: []byte{0x2D}, Last: []byte{0x2F}, Value: 14},
		},
	}
)

// TestExtractCMap tests that a CMap can be extracted from a CMAP file
// and that the stream dictionary is read correctly.
func TestExtractCMAP(t *testing.T) {
	// Write a CMap "by hand".

	data, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)

	rosRef, err := rm.EmbedFunc(font.WriteCIDSystemInfo, testROS)
	if err != nil {
		t.Fatal(err)
	}

	parentRef := data.Alloc()
	parentDict := pdf.Dict{
		"Type":          pdf.Name("CMap"),
		"CMapName":      pdf.Name("TestH"),
		"CIDSystemInfo": rosRef,
	}
	stm, err := data.OpenStream(parentRef, parentDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stm.Write(testCMapParent)
	if err != nil {
		t.Fatal(err)
	}
	err = stm.Close()
	if err != nil {
		t.Fatal(err)
	}

	childRef := data.Alloc()
	cihldDict := pdf.Dict{
		"Type":          pdf.Name("CMap"),
		"CMapName":      pdf.Name("TestV"),
		"CIDSystemInfo": rosRef,
		"WMode":         pdf.Integer(1),
		"UseCMap":       parentRef,
	}
	stm, err = data.OpenStream(childRef, cihldDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stm.Write(testCMapChild)
	if err != nil {
		t.Fatal(err)
	}
	err = stm.Close()
	if err != nil {
		t.Fatal(err)

	}

	child, err := Extract(pdf.NewCursor(data), childRef, false)
	if err != nil {
		t.Fatal(err)
	}

	if child.Name != "TestV" {
		t.Errorf("expected name %q, got %q", "TestV", child.Name)
	}
	if child.WMode != font.Vertical {
		t.Errorf("expected writing mode %v, got %v", font.Vertical, child.WMode)
	}
	if !reflect.DeepEqual(child.ROS, testROS) {
		t.Errorf("unexpected ROS: %v", child.ROS)
	}
	if child.Parent == nil {
		t.Errorf("expected parent, got nil")
		return
	}

	parent := child.Parent
	if parent.Name != "TestH" {
		t.Errorf("expected parent name %q, got %q", "TestH", parent.Name)
	}
	if parent.WMode != font.Horizontal {
		t.Errorf("expected parent writing mode %v, got %v", font.Horizontal, parent.WMode)
	}
	if !reflect.DeepEqual(parent.ROS, testROS) {
		t.Errorf("unexpected parent ROS: %v", parent.ROS)
	}
	if parent.Parent != nil {
		t.Errorf("expected no parent, got %v", parent.Parent)
	}

	if !reflect.DeepEqual(parent.CodeSpaceRange,
		charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}}) {
		t.Errorf("unexpected code space range: %v", parent.CodeSpaceRange)
	}
	if !reflect.DeepEqual(parent.CIDSingles,
		[]Single{{Code: []byte{0x20}, Value: 1}}) {
		t.Errorf("unexpected CID singles: %v", parent.CIDSingles)
	}
}

// TestReadCMap tests that all fields of a CMap can be read correctly
// from a CMAP file.
func TestReadCMap(t *testing.T) {
	info, parent, err := readCMap(bytes.NewReader(testCMapFull))
	if err != nil {
		t.Fatal(err)
	}

	if info.Name != "Test1" {
		t.Errorf("expected name %q, got %q", "Test1", info.Name)
	}

	if info.WMode != font.Vertical {
		t.Errorf("expected writing mode %v, got %v", font.Horizontal, info.WMode)
	}

	if parent != pdf.Name("Test2") {
		t.Errorf("expected parent %q, got %q", "Test2", parent)
	}

	if info.ROS.Registry != "Seehuhn" {
		t.Errorf("expected registry %q, got %q", "Seehuhn", info.ROS.Registry)
	}
	if info.ROS.Ordering != "Test" {
		t.Errorf("expected ordering %q, got %q", "Test", info.ROS.Ordering)
	}
	if info.ROS.Supplement != 42 {
		t.Errorf("expected supplement %d, got %d", 42, info.ROS.Supplement)
	}

	if !reflect.DeepEqual(info.CodeSpaceRange,
		charcode.CodeSpaceRange{{Low: []byte{0x20}, High: []byte{0x7F}}}) {
		t.Errorf("unexpected code space range: %v", info.CodeSpaceRange)
	}

	if !reflect.DeepEqual(info.CIDSingles,
		[]Single{{Code: []byte{0x20}, Value: 1}}) {
		t.Errorf("unexpected CID singles: %v", info.CIDSingles)
	}

	if !reflect.DeepEqual(info.CIDRanges,
		[]Range{{First: []byte{0x22}, Last: []byte{0x24}, Value: 3}}) {
		t.Errorf("unexpected CID ranges: %v", info.CIDRanges)
	}

	if !reflect.DeepEqual(info.NotdefSingles,
		[]Single{{Code: []byte{0x21}, Value: 2}}) {
		t.Errorf("unexpected notdef singles: %v", info.NotdefSingles)
	}

	if !reflect.DeepEqual(info.NotdefRanges,
		[]Range{{First: []byte{0x25}, Last: []byte{0x27}, Value: 6}}) {
		t.Errorf("unexpected notdef ranges: %v", info.NotdefRanges)
	}
}

// FuzzReadCMap tests that there is a bijection between textual CMap files,
// and the Info struct (ignoring the parent CMap name, if any).
func FuzzReadCMap(f *testing.F) {
	// Add all test CMaps from above
	f.Add(testCMapParent)
	f.Add(testCMapChild)
	f.Add(testCMapFull)

	buf := &bytes.Buffer{}
	for _, info := range []*File{testInfoFull, testInfoParent, testInfoChild} {
		buf.Reset()
		err := cmapTmplNew.Execute(buf, templateData{File: info})
		if err != nil {
			f.Fatal(err)
		}
		f.Add(bytes.Clone(buf.Bytes()))
	}

	// Also add all predefined CMaps
	for _, name := range allPredefined {
		fd, err := openPredefined(name)
		if err != nil {
			f.Fatal(err)
		}
		body, err := io.ReadAll(fd)
		if err != nil {
			f.Fatal(err)
		}
		err = fd.Close()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(body)
	}

	// ToUnicode CMaps are not valid here, but since they are very similar
	// in structure we add them to the corpus, too.
	f.Add(testToUniCMapParent)
	f.Add(testToUniCMapChild)
	f.Add(testToUniCMapFull)

	f.Fuzz(func(t *testing.T, body []byte) {
		info, _, err := readCMap(bytes.NewReader(body))
		if err != nil {
			t.Skip(err)
		}

		buf := &bytes.Buffer{}
		err = cmapTmplNew.Execute(buf, templateData{File: info})
		if err != nil {
			t.Fatal(err)
		}

		info2, _, err := readCMap(buf)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(info, info2) {
			t.Errorf("CMaps not equal: %s", cmp.Diff(info, info2))
		}
	})
}

// TestExtractPredefined tests that the predefined CMaps can be extracted
// without error and have the correct writing mode.
func TestExtractPredefined(t *testing.T) {
	names := []pdf.Name{
		// Chinese (simplified)
		"GB-EUC-H",
		"GB-EUC-V",
		"GBpc-EUC-H",
		"GBpc-EUC-V",
		"GBK-EUC-H",
		"GBK-EUC-V",
		"GBKp-EUC-H",
		"GBKp-EUC-V",
		"GBK2K-H",
		"GBK2K-V",
		"UniGB-UCS2-H",
		"UniGB-UCS2-V",
		"UniGB-UTF16-H",
		"UniGB-UTF16-V",

		// Chinese (traditional)
		"B5pc-H",
		"B5pc-V",
		"HKscs-B5-H",
		"HKscs-B5-V",
		"ETen-B5-H",
		"ETen-B5-V",
		"ETenms-B5-H",
		"ETenms-B5-V",
		"CNS-EUC-H",
		"CNS-EUC-V",
		"UniCNS-UCS2-H",
		"UniCNS-UCS2-V",
		"UniCNS-UTF16-H",
		"UniCNS-UTF16-V",

		// Japanese
		"83pv-RKSJ-H",
		"90ms-RKSJ-H",
		"90ms-RKSJ-V",
		"90msp-RKSJ-H",
		"90msp-RKSJ-V",
		"90pv-RKSJ-H",
		"Add-RKSJ-H",
		"Add-RKSJ-V",
		"EUC-H",
		"EUC-V",
		"Ext-RKSJ-H",
		"Ext-RKSJ-V",
		"H",
		"V",
		"UniJIS-UCS2-H",
		"UniJIS-UCS2-V",
		"UniJIS-UCS2-HW-H",
		"UniJIS-UCS2-HW-V",
		"UniJIS-UTF16-H",
		"UniJIS-UTF16-V",

		// Korean
		"KSC-EUC-H",
		"KSC-EUC-V",
		"KSCms-UHC-H",
		"KSCms-UHC-V",
		"KSCms-UHC-HW-H",
		"KSCms-UHC-HW-V",
		"KSCpc-EUC-H",
		"UniKS-UCS2-H",
		"UniKS-UCS2-V",
		"UniKS-UTF16-H",
		"UniKS-UTF16-V",

		// Others
		"Identity-H",
		"Identity-V",
	}
	for _, name := range names {
		data, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		t.Run(string(name), func(t *testing.T) {
			info, err := Extract(pdf.NewCursor(data), name, false)
			if err != nil {
				t.Fatal(err)
			}

			if strings.HasSuffix(info.Name, "H") {
				if info.WMode != font.Horizontal {
					t.Errorf("expected horizontal writing mode, got %v", info.WMode)
				}
			} else {
				if info.WMode != font.Vertical {
					t.Errorf("expected vertical writing mode, got %v", info.WMode)
				}
			}
		})
	}
}

// TestExtractLoop makes sure that the reader correctly handles loops in the
// UseCMap chain.
func TestExtractLoop(t *testing.T) {
	// Try different loop lengths:
	for n := 1; n <= 3; n++ {
		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			data, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(data)
			ros := &cid.SystemInfo{
				Registry:   "Test",
				Ordering:   "Simple",
				Supplement: 0,
			}
			rosRef, err := rm.EmbedFunc(font.WriteCIDSystemInfo, ros)
			if err != nil {
				t.Fatal(err)
			}

			// construct a loop of n CMaps
			refs := make([]pdf.Reference, n)
			cmaps := make([]*File, n)
			for i := range refs {
				refs[i] = data.Alloc()

				cmaps[i] = &File{
					Name: fmt.Sprintf("Test%d", i),
					ROS:  ros,
					CodeSpaceRange: charcode.CodeSpaceRange{
						{Low: []byte{0x00}, High: []byte{0xFF}},
					},
					CIDSingles: []Single{
						{Code: []byte{0x02 + byte(i)}, Value: cid.CID(1 + i)},
					},
				}
			}
			for i := range cmaps {
				cmaps[i].Parent = cmaps[(i+1)%n]
			}
			for i := range n {
				dict := pdf.Dict{
					"Type":          pdf.Name("CMap"),
					"CMapName":      pdf.Name(cmaps[i].Name),
					"CIDSystemInfo": rosRef,
					"UseCMap":       refs[(i+1)%n],
				}
				stm, err := data.OpenStream(refs[i], dict)
				if err != nil {
					t.Fatal(err)
				}
				err = cmapTmplNew.Execute(stm, templateData{File: cmaps[i]})
				if err != nil {
					t.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					t.Fatal(err)
				}
			}

			info, err := pdf.NewCursor(data).Decode(refs[0], Extract)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < n; i++ {
				// check that we got the correct CMap
				if info.Name != cmaps[i].Name {
					t.Errorf("expected name %q, got %q", cmaps[i].Name, info.Name)
				}

				// make sure the parent chain is correct
				if i < n-1 {
					if info.Parent == nil {
						t.Fatalf("expected parent, got nil")
					}
					info = info.Parent
				} else {
					if info.Parent != nil {
						t.Fatalf("expected no parent, got %v", info.Parent.Name)
					}
				}
			}
		})
	}
}

func TestEmbedCMap(t *testing.T) {
	data, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)
	ref, err := rm.Embed(testToUniInfoChild)
	if err != nil {
		t.Fatal(err)
	}

	info2, err := pdf.NewCursor(data).Decode(ref, ExtractToUnicode)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(testToUniInfoChild, info2) {
		t.Errorf("expected %v, got %v", testToUniInfoChild, info2)
	}
}

func TestCMapTemplate(t *testing.T) {
	buf := &bytes.Buffer{}

	// check that the template renders without error
	err := cmapTmplNew.Execute(buf, templateData{File: testInfoFull})
	if err != nil {
		t.Fatal(err)
	}

	// check that some key lines are present in the output
	body := buf.String()
	lines := []string{
		"/Other usecmap",
		"/CMapName /Test def",
		"/WMode 0 def",
		"2 begincodespacerange\n<00> <fe>\n<ff00> <ffff>\nendcodespacerange",
		"2 begincidchar\n<20> 2\n<22> 3\nendcidchar",
		"1 begincidrange\n<ff20> <ffff> 5\nendcidrange",
		"1 beginnotdefchar\n<21> 1\nendnotdefchar",
		"2 beginnotdefrange\n<00> <1f> 0\n<ff00> <ff1f> 4\nendnotdefrange",
	}
	for _, line := range lines {
		if !strings.Contains(body, line) {
			t.Errorf("expected line %q not found in output", line)
		}
	}
}

// TestExtractCMapOversize verifies that a CMap stream larger than
// limits.MaxCMapBytes is rejected, blocking a decompression-bomb
// attack on CMap loading.
func TestExtractCMapOversize(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, pdf.Dict{
		"Type":     pdf.Name("CMap"),
		"CMapName": pdf.Name("TestH"),
	})
	if err != nil {
		t.Fatal(err)
	}
	// write more than MaxCMapBytes of payload
	body := make([]byte, limits.MaxCMapBytes+1)
	if _, err := stm.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Extract(pdf.NewCursor(w), ref, false); err == nil {
		t.Fatal("expected error for oversize CMap, got nil")
	}
}

// TestLookupPrecedence checks the precedence of CID and notdef mappings,
// within one CMap and across a usecmap chain, and that it agrees with All.
func TestLookupPrecedence(t *testing.T) {
	parent := &File{
		CodeSpaceRange: charcode.Simple,
		CIDRanges:      []Range{{First: []byte{0x40}, Last: []byte{0x4F}, Value: 100}},
		NotdefRanges:   []Range{{First: []byte{0x00}, Last: []byte{0x1F}, Value: 1}},
	}
	child := &File{
		CIDSingles: []Single{
			{Code: []byte{0x45}, Value: 300},
			{Code: []byte{0x45}, Value: 301},
		},
		CIDRanges: []Range{
			{First: []byte{0x42}, Last: []byte{0x46}, Value: 200},
			{First: []byte{0x44}, Last: []byte{0x48}, Value: 210},
		},
		NotdefSingles: []Single{{Code: []byte{0x05}, Value: 2}},
		NotdefRanges: []Range{
			{First: []byte{0x10}, Last: []byte{0x17}, Value: 3},
			{First: []byte{0x14}, Last: []byte{0x1B}, Value: 4},
		},
		Parent: parent,
	}

	cases := []struct {
		code        byte
		cid, notdef cid.CID
	}{
		{0x41, 101, 0}, // parent range
		{0x42, 200, 0}, // child range overrides parent
		{0x44, 210, 0}, // later range overrides earlier
		{0x45, 301, 0}, // later single overrides ranges and earlier single
		{0x49, 109, 0}, // parent range
		{0x05, 2, 2},   // child notdef single
		{0x06, 1, 1},   // parent notdef range
		{0x11, 3, 3},   // child notdef range overrides parent
		{0x15, 4, 4},   // later notdef range overrides earlier
		{0x1C, 1, 1},   // parent notdef range
		{0x30, 0, 0},   // nothing
	}
	for _, c := range cases {
		if got := child.LookupCID([]byte{c.code}); got != c.cid {
			t.Errorf("LookupCID(%02x) = %d, want %d", c.code, got, c.cid)
		}
		if got := child.LookupNotdefCID([]byte{c.code}); got != c.notdef {
			t.Errorf("LookupNotdefCID(%02x) = %d, want %d", c.code, got, c.notdef)
		}
	}

	codec, err := child.Codec()
	if err != nil {
		t.Fatal(err)
	}
	for code, c := range maps.Collect(child.All(codec)) {
		if got := child.LookupCID(codec.AppendCode(nil, code)); got != c {
			t.Errorf("code %02x: All gives CID %d, LookupCID %d", code, c, got)
		}
	}
}

// TestFileNil checks that the lookup methods accept a nil receiver.
func TestFileNil(t *testing.T) {
	var f *File
	if c := f.LookupCID([]byte{0}); c != 0 {
		t.Errorf("nil LookupCID gave CID %d", c)
	}
	if c := f.LookupNotdefCID([]byte{0}); c != 0 {
		t.Errorf("nil LookupNotdefCID gave CID %d", c)
	}
	for range f.All(charcode.SimpleCodec) {
		t.Error("nil All yielded a mapping")
	}
	if f.Chain() != nil {
		t.Error("nil chain is not nil")
	}
}

// TestChainCycle checks that a cycle of Parent links, which a file cannot
// produce but a caller can construct, lists each CMap once.
func TestChainCycle(t *testing.T) {
	a := &File{CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0x7F}}}}
	b := &File{CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x80, 0x00}, High: []byte{0xFF, 0xFF}}}}
	a.Parent = b
	b.Parent = a

	chain := a.Chain()
	if len(chain) != 2 || chain[0] != a || chain[1] != b {
		t.Errorf("wrong chain %v", chain)
	}
	if (*File)(nil).Chain() != nil {
		t.Error("nil chain is not nil")
	}

	codec, err := a.Codec()
	if err != nil {
		t.Fatal(err)
	}
	if _, k, valid := codec.Decode([]byte{0x80, 0x01}); !valid || k != 2 {
		t.Errorf("two-byte code: k=%d, valid=%t", k, valid)
	}
}

// TestRangeCIDOverflow checks that the codes of a range whose CID would
// exceed the largest CID are not mapped by that range, consistently in
// LookupCID and All, and that the file reads back the same after writing.
func TestRangeCIDOverflow(t *testing.T) {
	parent := &File{
		CodeSpaceRange: charcode.Simple,
		CIDSingles:     []Single{{Code: []byte{0x02}, Value: 7}},
	}
	f := &File{
		Name:           "Overflow",
		ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
		CodeSpaceRange: charcode.Simple,
		CIDRanges: []Range{
			{First: []byte{0x00}, Last: []byte{0xFF}, Value: math.MaxUint32 - 1},
		},
		Parent: parent,
	}

	want := map[byte]cid.CID{
		0x00: math.MaxUint32 - 1,
		0x01: math.MaxUint32,
		0x02: 7, // from the parent
		0x03: 0, // unmapped
		0xFF: 0, // unmapped
	}
	for code, c := range want {
		if got := f.LookupCID([]byte{code}); got != c {
			t.Errorf("LookupCID(%02x) = %d, want %d", code, got, c)
		}
	}

	codec, err := f.Codec()
	if err != nil {
		t.Fatal(err)
	}
	got := maps.Collect(f.All(codec))
	wantAll := map[charcode.Code]cid.CID{0x00: math.MaxUint32 - 1, 0x01: math.MaxUint32, 0x02: 7}
	if diff := cmp.Diff(wantAll, got); diff != "" {
		t.Errorf("All (-want +got):\n%s", diff)
	}

	buf := &bytes.Buffer{}
	if err := f.WriteTo(buf, false); err != nil {
		t.Fatal(err)
	}
	f2, _, err := readCMap(buf)
	if err != nil {
		t.Fatal(err)
	}
	f2.Parent = parent
	for code := range 256 {
		c := []byte{byte(code)}
		if a, b := f.LookupCID(c), f2.LookupCID(c); a != b {
			t.Errorf("code %02x: CID %d before writing, %d after", code, a, b)
		}
	}
}

// Malformed code space ranges are dropped on read, so that the remaining
// ranges still give a codec.
func TestReadCMapInvalidCodeSpace(t *testing.T) {
	data := func(cmapType int) []byte {
		return fmt.Appendf(nil, `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Test def
/CMapType %d def
3 begincodespacerange
<00> <7F>
<80> <20>
<8000000000> <FF00000000>
endcodespacerange
1 begincidchar
<20> 1
endcidchar
endcmap
CMapName currentdict /CMap defineresource pop
end
end
`, cmapType)
	}
	want := charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0x7F}}}

	info, _, err := readCMap(bytes.NewReader(data(1)))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, info.CodeSpaceRange); diff != "" {
		t.Errorf("wrong code space ranges (-want +got):\n%s", diff)
	}
	if _, err := info.Codec(); err != nil {
		t.Errorf("no codec: %v", err)
	}

	tu, err := readToUnicode(bytes.NewReader(data(2)))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, tu.CodeSpaceRange); diff != "" {
		t.Errorf("wrong ToUnicode code space ranges (-want +got):\n%s", diff)
	}
}
