// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cmap

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

var _ pdf.Embedder[pdf.Unused] = (*InfoNew)(nil)

// TestExtractCMap tests that a CMap can be extracted from a CMAP file
// and that the stream dictionary is read correctly.
func TestExtractCMAP(t *testing.T) {
	// Write a CMap "by hand".
	testH := `/CIDInit /ProcSet findresource begin
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
`
	testV := `/CIDInit /ProcSet findresource begin
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
`

	data := pdf.NewData(pdf.V2_0)
	rm := pdf.NewResourceManager(data)

	ros := &CIDSystemInfo{
		Registry:   "Test",
		Ordering:   "Simple",
		Supplement: 0,
	}
	rosRef, _, err := pdf.ResourceManagerEmbed(rm, ros)
	if err != nil {
		t.Fatal(err)
	}

	testHRef := data.Alloc()
	testHDict := pdf.Dict{
		"Type":          pdf.Name("CMap"),
		"CMapName":      pdf.Name("TestH"),
		"CIDSystemInfo": rosRef,
	}
	stm, err := data.OpenStream(testHRef, testHDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stm.Write([]byte(testH))
	if err != nil {
		t.Fatal(err)
	}
	err = stm.Close()
	if err != nil {
		t.Fatal(err)
	}

	testVRef := data.Alloc()
	testVDict := pdf.Dict{
		"Type":          pdf.Name("CMap"),
		"CMapName":      pdf.Name("TestV"),
		"CIDSystemInfo": rosRef,
		"WMode":         pdf.Integer(1),
		"UseCMap":       testHRef,
	}
	stm, err = data.OpenStream(testVRef, testVDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stm.Write([]byte(testV))
	if err != nil {
		t.Fatal(err)
	}
	err = stm.Close()
	if err != nil {
		t.Fatal(err)

	}

	info, err := ExtractNew(data, testVRef)
	if err != nil {
		t.Fatal(err)
	}

	if info.Name != "TestV" {
		t.Errorf("expected name %q, got %q", "TestV", info.Name)
	}
	if info.WMode != Vertical {
		t.Errorf("expected writing mode %v, got %v", Vertical, info.WMode)
	}
	if !reflect.DeepEqual(info.ROS, ros) {
		t.Errorf("unexpected ROS: %v", info.ROS)
	}
	if info.Parent == nil {
		t.Errorf("expected parent, got nil")
		return
	}

	parent := info.Parent
	if parent.Name != "TestH" {
		t.Errorf("expected parent name %q, got %q", "TestH", parent.Name)
	}
	if parent.WMode != Horizontal {
		t.Errorf("expected parent writing mode %v, got %v", Horizontal, parent.WMode)
	}
	if !reflect.DeepEqual(parent.ROS, ros) {
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
		[]SingleNew{{Code: []byte{0x20}, Value: 1}}) {
		t.Errorf("unexpected CID singles: %v", parent.CIDSingles)
	}
}

// TestReadCMap tests that all fields of a CMap can be read correctly
// from a CMAP file.
func TestReadCMap(t *testing.T) {
	in := `/CIDInit /ProcSet findresource begin
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
`
	info, parent, err := readCMap(bytes.NewReader([]byte(in)))
	if err != nil {
		t.Fatal(err)
	}

	if info.Name != "Test1" {
		t.Errorf("expected name %q, got %q", "Test1", info.Name)
	}

	if info.WMode != Vertical {
		t.Errorf("expected writing mode %v, got %v", Horizontal, info.WMode)
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
		[]SingleNew{{Code: []byte{0x20}, Value: 1}}) {
		t.Errorf("unexpected CID singles: %v", info.CIDSingles)
	}

	if !reflect.DeepEqual(info.CIDRanges,
		[]RangeNew{{First: []byte{0x22}, Last: []byte{0x24}, Value: 3}}) {
		t.Errorf("unexpected CID ranges: %v", info.CIDRanges)
	}

	if !reflect.DeepEqual(info.NotdefSingles,
		[]SingleNew{{Code: []byte{0x21}, Value: 2}}) {
		t.Errorf("unexpected notdef singles: %v", info.NotdefSingles)
	}

	if !reflect.DeepEqual(info.NotdefRanges,
		[]RangeNew{{First: []byte{0x25}, Last: []byte{0x27}, Value: 6}}) {
		t.Errorf("unexpected notdef ranges: %v", info.NotdefRanges)
	}
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
		data := pdf.NewData(pdf.V2_0)
		t.Run(string(name), func(t *testing.T) {
			info, err := ExtractNew(data, name)
			if err != nil {
				t.Fatal(err)
			}

			if strings.HasSuffix(string(info.Name), "H") {
				if info.WMode != Horizontal {
					t.Errorf("expected horizontal writing mode, got %v", info.WMode)
				}
			} else {
				if info.WMode != Vertical {
					t.Errorf("expected vertical writing mode, got %v", info.WMode)
				}
			}
		})
	}
}

// TestExtractLoop makes sure that the reader correctly handles loops in the
// UseCMap chain.
func TestExtractLoop(t *testing.T) {
	for n := 1; n <= 3; n++ {
		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			data := pdf.NewData(pdf.V2_0)
			rm := pdf.NewResourceManager(data)
			ros := &CIDSystemInfo{
				Registry:   "Test",
				Ordering:   "Simple",
				Supplement: 0,
			}
			rosRef, _, err := pdf.ResourceManagerEmbed(rm, ros)
			if err != nil {
				t.Fatal(err)
			}

			// construct a loop of n CMaps
			refs := make([]pdf.Reference, n)
			for i := range refs {
				refs[i] = data.Alloc()
			}
			for i := range n {
				thisName := fmt.Sprintf("Test%d", i)
				nextName := fmt.Sprintf("Test%d", (i+1)%n)
				// Write a CMap "by hand".
				body := `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap

/CMapName /` + thisName + ` def
/CMapType 1 def
/WMode 0 def

/` + nextName + ` usecmap

/CIDSystemInfo 3 dict dup begin
  /Registry (Test) def
  /Ordering (Qwerty) def
  /Supplement 0 def
end def

endcmap
CMapName currentdict /CMap defineresource pop
end
end`
				dict := pdf.Dict{
					"Type":          pdf.Name("CMap"),
					"CMapName":      pdf.Name(thisName),
					"CIDSystemInfo": rosRef,
					"UseCMap":       refs[(i+1)%n],
				}
				stm, err := data.OpenStream(refs[i], dict)
				if err != nil {
					t.Fatal(err)
				}
				_, err = stm.Write([]byte(body))
				if err != nil {
					t.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					t.Fatal(err)
				}
			}

			info, err := ExtractNew(data, refs[0])
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < n; i++ {
				expected := pdf.Name(fmt.Sprintf("Test%d", i))
				if info.Name != expected {
					t.Errorf("expected name %q, got %q", expected, info.Name)
				}

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

func TestCMapTemplate(t *testing.T) {
	info := &InfoNew{
		Name: "Test",
		ROS: &CIDSystemInfo{
			Registry:   "Test",
			Ordering:   "Random",
			Supplement: 3,
		},
		WMode: Horizontal,
		Parent: &InfoNew{
			Name: "Other",
		},
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00}, High: []byte{0xFE}},
			{Low: []byte{0xFF, 0x00}, High: []byte{0xFF, 0xFF}},
		},
		CIDSingles: []SingleNew{
			{Code: []byte{0x20}, Value: 2},
			{Code: []byte{0x22}, Value: 3},
		},
		CIDRanges: []RangeNew{
			{First: []byte{0xFF, 0x20}, Last: []byte{0xFF, 0xFF}, Value: 5},
		},
		NotdefSingles: []SingleNew{
			{Code: []byte{0x21}, Value: 1},
		},
		NotdefRanges: []RangeNew{
			{First: []byte{0x00}, Last: []byte{0x1F}, Value: 0},
			{First: []byte{0xFF, 0x00}, Last: []byte{0xFF, 0x1F}, Value: 4},
		},
	}

	buf := &bytes.Buffer{}

	// check that the template renders without error
	err := cmapTmplNew.Execute(buf, info)
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

func TestEmbedCMap(t *testing.T) {
	ros := &CIDSystemInfo{
		Registry:   "Seehuhn",
		Ordering:   "Test",
		Supplement: 123,
	}
	parent := &InfoNew{
		Name:  "Test1",
		ROS:   ros,
		WMode: Vertical,
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00, 0x00}, High: []byte{0x00, 0xFF}},
		},
		CIDSingles: []SingleNew{
			{Code: []byte{0x20}, Value: 1},
		},
		CIDRanges: []RangeNew{
			{First: []byte{0x21}, Last: []byte{0x23}, Value: 2},
		},
		NotdefSingles: []SingleNew{
			{Code: []byte{0x24}, Value: 5},
		},
		NotdefRanges: []RangeNew{
			{First: []byte{0x25}, Last: []byte{0x27}, Value: 6},
		},
	}
	info := &InfoNew{
		Name:   "Test2",
		ROS:    ros,
		WMode:  Vertical,
		Parent: parent,
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00, 0x00}, High: []byte{0x00, 0xFF}},
		},
		CIDSingles: []SingleNew{
			{Code: []byte{0x28}, Value: 9},
		},
		CIDRanges: []RangeNew{
			{First: []byte{0x29}, Last: []byte{0x2B}, Value: 10},
		},
		NotdefSingles: []SingleNew{
			{Code: []byte{0x2C}, Value: 13},
		},
		NotdefRanges: []RangeNew{
			{First: []byte{0x2D}, Last: []byte{0x2F}, Value: 14},
		},
	}

	data := pdf.NewData(pdf.V2_0)
	rm := pdf.NewResourceManager(data)
	ref, _, err := pdf.ResourceManagerEmbed(rm, info)
	if err != nil {
		t.Fatal(err)
	}

	info2, err := ExtractNew(data, ref)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(info, info2) {
		t.Errorf("expected %v, got %v", info, info2)
	}
}

// FuzzReadCMap tests that there is a bijections between textual CMap files,
// and the Info struct (ignoring the parent CMap name, if any).
func FuzzReadCMap(f *testing.F) {
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

	info := &InfoNew{
		Name: "Test",
		ROS: &CIDSystemInfo{
			Registry:   "Test",
			Ordering:   "Random",
			Supplement: 3,
		},
		WMode: Horizontal,
		Parent: &InfoNew{
			Name: "Other",
		},
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00}, High: []byte{0xFE}},
			{Low: []byte{0xFF, 0x00}, High: []byte{0xFF, 0xFF}},
		},
		CIDSingles: []SingleNew{
			{Code: []byte{0x20}, Value: 2},
			{Code: []byte{0x22}, Value: 3},
		},
		CIDRanges: []RangeNew{
			{First: []byte{0xFF, 0x20}, Last: []byte{0xFF, 0xFF}, Value: 5},
		},
		NotdefSingles: []SingleNew{
			{Code: []byte{0x21}, Value: 1},
		},
		NotdefRanges: []RangeNew{
			{First: []byte{0x00}, Last: []byte{0x1F}, Value: 0},
			{First: []byte{0xFF, 0x00}, Last: []byte{0xFF, 0x1F}, Value: 4},
		},
	}
	buf := &bytes.Buffer{}
	err := cmapTmplNew.Execute(buf, info)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, body []byte) {
		info, _, err := readCMap(bytes.NewReader(body))
		if err != nil {
			t.Skip(err)
		}

		buf := &bytes.Buffer{}
		err = cmapTmplNew.Execute(buf, info)
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
