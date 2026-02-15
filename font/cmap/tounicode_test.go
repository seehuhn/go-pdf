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
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var _ pdf.Embedder = (*ToUnicodeFile)(nil)

func TestToUnicodeRangeIter(t *testing.T) {
	type testCase struct {
		r    ToUnicodeRange
		want map[charcode.Code]string
	}
	cases := []testCase{
		{
			r: ToUnicodeRange{
				First: []byte{0xF0},
				Last:  []byte{0xF2},
				Values: []string{
					"fl",
					"fi",
					"ffl",
				},
			},
			want: map[charcode.Code]string{
				0xF0: "fl",
				0xF1: "fi",
				0xF2: "ffl",
			},
		},
		{
			r: ToUnicodeRange{
				First:  []byte{0x41},
				Last:   []byte{0x44},
				Values: []string{"A"},
			},
			want: map[charcode.Code]string{
				0x41: "A",
				0x42: "B",
				0x43: "C",
				0x44: "D",
			},
		},
	}

	for _, c := range cases {
		tu := &ToUnicodeFile{
			CodeSpaceRange: charcode.Simple,
			Ranges:         []ToUnicodeRange{c.r},
		}
		got, err := tu.GetMapping()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("unexpected result: got %v, want %v", got, c.want)
		}
	}
}

// The following variabes contain test CMap data used in the tests below.
var (
	testToUniCMapParent = []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def
/CMapName /Parent def
/CMapType 2 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
2 beginbfrange
<0000> <005E> <0020>
<005F> <0061> [<00660066> <00660069> <00660066006C>]
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end
`)
	testToUniCMapChild = []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def
/CMapName /Child def
/CMapType 2 def
/Parent usecmap
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfchar
<3A51> <D840DC3E>
endbfchar
endcmap
CMapName currentdict /CMap defineresource pop
end
end
`)
	testToUniCMapFull = []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def
/CMapName /Full def
/CMapType 2 def
/Parent usecmap
2 begincodespacerange
<00> <7F>
<8000> <FFFF>
endcodespacerange
2 beginbfchar
<02> <0041>
<8001> <0042>
endbfchar
3 beginbfrange
<03> <05> <0043>
<8002> <8004> [<0045> <0044> <0046>]
<8005> <8007> <0047>
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end
`)

	testToUniInfoParent = &ToUnicodeFile{
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00}, High: []byte{0xFE}},
			{Low: []byte{0xFF, 0x00}, High: []byte{0xFF, 0xFE}},
			{Low: []byte{0xFF, 0xFF, 0x00}, High: []byte{0xFF, 0xFF, 0xFF}},
		},
		Singles: []ToUnicodeSingle{
			{Code: []byte{0x02}, Value: "A"},
			{Code: []byte{0x04}, Value: "C"},
			{Code: []byte{0x05}, Value: "日"},
		},
		Ranges: []ToUnicodeRange{
			{
				First:  []byte{0x07},
				Last:   []byte{0x09},
				Values: []string{"G", "H", "I"},
			},
		},
	}
	testToUniInfoChild = &ToUnicodeFile{
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00}, High: []byte{0xFE}},
			{Low: []byte{0xFF, 0x00}, High: []byte{0xFF, 0xFE}},
			{Low: []byte{0xFF, 0xFF, 0x00}, High: []byte{0xFF, 0xFF, 0xFF}},
		},
		Singles: []ToUnicodeSingle{
			{Code: []byte{0x02}, Value: "A"},
			{Code: []byte{0x04}, Value: "C"},
			{Code: []byte{0x05}, Value: "日"},
		},
		Ranges: []ToUnicodeRange{
			{
				First:  []byte{0x07},
				Last:   []byte{0x09},
				Values: []string{"G", "H", "I"},
			},
			{
				First:  []byte{0xFF, 0x10},
				Last:   []byte{0xFF, 0x20},
				Values: []string{"XA"},
			},
		},
		Parent: testToUniInfoParent,
	}
)

func TestMakeName(t *testing.T) {
	name1 := testToUniInfoParent.MakeName()
	name2 := testToUniInfoChild.MakeName()

	namePat := regexp.MustCompile(`^seehuhn-[0-9a-f]{32}-UTF16$`)
	for _, name := range []pdf.Name{name1, name2} {
		if !namePat.MatchString(string(name)) {
			t.Errorf("invalid name: %q", name)
		}
	}

	if name1 == name2 {
		t.Errorf("name1 and name2 should be different")
	}
}

func BenchmarkMakeName(b *testing.B) {
	for range b.N {
		testToUniInfoChild.MakeName()
	}
}

func TestExtractToUnicode(t *testing.T) {
	// Write a ToUnicode CMap "by hand".

	data, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)

	rosRef, err := pdf.ResourceManagerEmbedFunc(rm, font.WriteCIDSystemInfo, toUnicodeROS)
	if err != nil {
		t.Fatal(err)
	}

	parentRef := data.Alloc()
	parentDict := pdf.Dict{
		"Type":          pdf.Name("CMap"),
		"CMapName":      pdf.Name("Parent"),
		"CIDSystemInfo": rosRef,
	}
	stm, err := data.OpenStream(parentRef, parentDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stm.Write(testToUniCMapParent)
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
		"CMapName":      pdf.Name("Child"),
		"CIDSystemInfo": rosRef,
		"UseCMap":       parentRef,
	}
	stm, err = data.OpenStream(childRef, cihldDict)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stm.Write(testToUniCMapChild)
	if err != nil {
		t.Fatal(err)
	}
	err = stm.Close()
	if err != nil {
		t.Fatal(err)

	}

	child, err := ExtractToUnicode(pdf.NewExtractor(data), childRef)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(child.CodeSpaceRange,
		charcode.CodeSpaceRange{{Low: []byte{0x00, 0x00}, High: []byte{0xFF, 0xFF}}}) {
		t.Errorf("unexpected code space range: %v", child.CodeSpaceRange)
	}
	if !reflect.DeepEqual(child.Singles,
		[]ToUnicodeSingle{
			{Code: []byte{0x3A, 0x51}, Value: "\U0002003E"},
		}) {
		t.Errorf("unexpected singles: %v", child.Singles)
	}
	if len(child.Ranges) != 0 {
		t.Errorf("unexpected ranges: %v", child.Ranges)
	}
	if child.Parent == nil {
		t.Fatal("child.Parent is nil")
	}

	parent := child.Parent
	if !reflect.DeepEqual(parent.CodeSpaceRange,
		charcode.CodeSpaceRange{{Low: []byte{0x00, 0x00}, High: []byte{0xFF, 0xFF}}}) {
		t.Errorf("unexpected code space range: %v", parent.CodeSpaceRange)
	}
	if len(parent.Singles) != 0 {
		t.Errorf("unexpected singles: %v", parent.Singles)
	}
	if !reflect.DeepEqual(parent.Ranges,
		[]ToUnicodeRange{
			{
				First:  []byte{0x00, 0x00},
				Last:   []byte{0x00, 0x5E},
				Values: []string{" "},
			},
			{
				First:  []byte{0x00, 0x5f},
				Last:   []byte{0x00, 0x61},
				Values: []string{"ff", "fi", "ffl"},
			},
		}) {
		t.Errorf("unexpected ranges: %v", parent.Ranges)
	}
	if parent.Parent != nil {
		t.Fatal("parent.Parent is not nil")
	}
}

func TestReadToUnicode(t *testing.T) {
	info, err := readToUnicode(bytes.NewReader(testToUniCMapFull))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(info.CodeSpaceRange,
		charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0x7F}},
			{Low: []byte{0x80, 0x00}, High: []byte{0xFF, 0xFF}}}) {
		t.Errorf("unexpected code space range: %v", info.CodeSpaceRange)
	}
	if !reflect.DeepEqual(info.Singles,
		[]ToUnicodeSingle{
			{Code: []byte{0x02}, Value: "A"},
			{Code: []byte{0x80, 0x01}, Value: "B"},
		}) {
		t.Errorf("unexpected singles: %v", info.Singles)
	}
	if !reflect.DeepEqual(info.Ranges,
		[]ToUnicodeRange{
			{
				First:  []byte{0x03},
				Last:   []byte{0x05},
				Values: []string{"C"},
			},
			{
				First:  []byte{0x80, 0x02},
				Last:   []byte{0x80, 0x04},
				Values: []string{"E", "D", "F"},
			},
			{
				First:  []byte{0x80, 0x05},
				Last:   []byte{0x80, 0x07},
				Values: []string{"G"},
			},
		}) {
		t.Errorf("unexpected ranges: %v", info.Ranges)
	}
}

// FuzzReadToUnicode tests that there is a bijection between textual CMap files,
// and the Info struct.
func FuzzReadToUnicode(f *testing.F) {
	// Add all test ToUnicode CMaps from above
	f.Add(testToUniCMapParent)
	f.Add(testToUniCMapChild)
	f.Add(testToUniCMapFull)

	buf := &bytes.Buffer{}
	for _, info := range []*ToUnicodeFile{testToUniInfoParent, testToUniInfoChild} {
		buf.Reset()
		err := toUnicodeTmplNew.Execute(buf, info)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(bytes.Clone(buf.Bytes()))
	}

	// Normal CMaps are not valid here, but since they are very similar
	// in structure we add them to the corpus, too.
	f.Add(testCMapParent)
	f.Add(testCMapChild)
	f.Add(testCMapFull)

	f.Fuzz(func(t *testing.T, body []byte) {
		info, err := readToUnicode(bytes.NewReader(body))
		if err != nil {
			t.Skip(err)
		}

		buf := &bytes.Buffer{}
		err = toUnicodeTmplNew.Execute(buf, info)
		if err != nil {
			t.Fatal(err)
		}

		info2, err := readToUnicode(buf)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(info, info2) {
			t.Errorf("ToUnicode CMaps not equal: %s", cmp.Diff(info, info2))
		}
	})
}

// TestExtractToUnicodeLoop tests that the reader does not enter an infinite loop
// when extracting a ToUnicode CMap that references itself.
func TestExtractToUnicodeLoop(t *testing.T) {
	// Try different loop lengths:
	for n := 1; n <= 3; n++ {
		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			data, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(data)
			rosRef, err := pdf.ResourceManagerEmbedFunc(rm, font.WriteCIDSystemInfo, toUnicodeROS)
			if err != nil {
				t.Fatal(err)
			}

			// construct a loop of n CMaps
			refs := make([]pdf.Reference, n)
			cmaps := make([]*ToUnicodeFile, n)
			for i := range refs {
				refs[i] = data.Alloc()

				cmaps[i] = &ToUnicodeFile{
					CodeSpaceRange: []charcode.Range{
						{Low: []byte{0x00}, High: []byte{0xFF}},
					},
					Singles: []ToUnicodeSingle{
						{Code: []byte{0x02 + byte(i)}, Value: string('A' + rune(i))},
					},
				}
			}
			for i := range cmaps {
				cmaps[i].Parent = cmaps[(i+1)%n]
			}
			for i := range n {
				dict := pdf.Dict{
					"Type":          pdf.Name("CMap"),
					"CMapName":      cmaps[i].MakeName(),
					"CIDSystemInfo": rosRef,
					"UseCMap":       refs[(i+1)%n],
				}
				stm, err := data.OpenStream(refs[i], dict)
				if err != nil {
					t.Fatal(err)
				}
				err = toUnicodeTmplNew.Execute(stm, cmaps[i])
				if err != nil {
					t.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					t.Fatal(err)
				}
			}

			info, err := ExtractToUnicode(pdf.NewExtractor(data), refs[0])
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < n; i++ {
				// Make sure that we got the correct CMap
				if !reflect.DeepEqual(info.Singles, cmaps[i].Singles) {
					t.Fatalf("unexpected info: %v", info)
				}

				// Make sure the parent chain is correct
				if i < n-1 {
					if info.Parent == nil {
						t.Fatalf("expected parent, got nil")
					}
					info = info.Parent
				} else {
					if info.Parent != nil {
						t.Fatalf("expected no parent, got %v", info.Parent)
					}
				}
			}
		})
	}
}

func TestEmbedToUnicode(t *testing.T) {
	data, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)

	ref, err := rm.Embed(testToUniInfoChild)
	if err != nil {
		t.Fatal(err)
	}

	info, err := ExtractToUnicode(pdf.NewExtractor(data), ref)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(info, testToUniInfoChild) {
		t.Errorf("unexpected info: %v", info)
	}
}

func TestToUnicodeTemplate(t *testing.T) {
	buf := &bytes.Buffer{}

	// check that the template renders without error
	err := toUnicodeTmplNew.Execute(buf, testToUniInfoChild)
	if err != nil {
		t.Fatal(err)
	}

	// check that that some key lines are present in the output
	body := buf.String()
	lines := []string{
		pdf.AsString(testToUniInfoParent.MakeName()) + " usecmap",
		"/CMapName " + pdf.AsString(testToUniInfoChild.MakeName()) + " def",
		"3 begincodespacerange\n<00> <fe>\n<ff00> <fffe>\n<ffff00> <ffffff>\nendcodespacerange",
		"3 beginbfchar\n<02> <0041>\n<04> <0043>\n<05> <65e5>\nendbfchar",
		"2 beginbfrange\n<07> <09> [<0047> <0048> <0049>]\n<ff10> <ff20> <00580041>\nendbfrange",
	}
	for _, line := range lines {
		if !strings.Contains(body, line) {
			t.Errorf("expected line %q not found in output", line)
		}
	}
}
