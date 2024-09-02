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
	"reflect"
	"regexp"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

var _ pdf.Embedder[pdf.Unused] = (*ToUnicodeInfo)(nil)

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

	testToUniInfoParent = &ToUnicodeInfo{
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00}, High: []byte{0xFE}},
			{Low: []byte{0xFF, 0x00}, High: []byte{0xFF, 0xFE}},
			{Low: []byte{0xFF, 0xFF, 0x00}, High: []byte{0xFF, 0xFF, 0xFF}},
		},
		Singles: []ToUnicodeSingle{
			{Code: []byte{0x02}, Value: []rune("A")},
			{Code: []byte{0x04}, Value: []rune("C")},
			{Code: []byte{0x05}, Value: []rune("日")},
		},
		Ranges: []ToUnicodeRange{
			{
				First:  []byte{0x07},
				Last:   []byte{0x09},
				Values: [][]rune{[]rune("G"), []rune("H"), []rune("I")},
			},
		},
	}
	testToUniInfoChild = &ToUnicodeInfo{
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00}, High: []byte{0xFE}},
			{Low: []byte{0xFF, 0x00}, High: []byte{0xFF, 0xFE}},
			{Low: []byte{0xFF, 0xFF, 0x00}, High: []byte{0xFF, 0xFF, 0xFF}},
		},
		Singles: []ToUnicodeSingle{
			{Code: []byte{0x02}, Value: []rune("A")},
			{Code: []byte{0x04}, Value: []rune("C")},
			{Code: []byte{0x05}, Value: []rune("日")},
		},
		Ranges: []ToUnicodeRange{
			{
				First:  []byte{0x07},
				Last:   []byte{0x09},
				Values: [][]rune{[]rune("G"), []rune("H"), []rune("I")},
			},
			{
				First:  []byte{0xFF, 0x10},
				Last:   []byte{0xFF, 0x20},
				Values: [][]rune{[]rune("XA")},
			},
		},
		Parent: testToUniInfoParent,
	}
)

func TestMakeName(t *testing.T) {
	name1 := testToUniInfoParent.MakeName()
	name2 := testToUniInfoChild.MakeName()

	namePat := regexp.MustCompile(`^GoPDF-[0-9a-f]{32}-UTF16$`)
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

	data := pdf.NewData(pdf.V2_0)
	rm := pdf.NewResourceManager(data)

	rosRef, _, err := pdf.ResourceManagerEmbed(rm, toUnicodeROS)
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

	child, err := ExtractToUnicodeNew(data, childRef)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(child.CodeSpaceRange,
		charcode.CodeSpaceRange{{Low: []byte{0x00, 0x00}, High: []byte{0xFF, 0xFF}}}) {
		t.Errorf("unexpected code space range: %v", child.CodeSpaceRange)
	}
	if !reflect.DeepEqual(child.Singles,
		[]ToUnicodeSingle{
			{Code: []byte{0x3A, 0x51}, Value: []rune("\U0002003E")},
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
				Values: [][]rune{[]rune(" ")},
			},
			{
				First:  []byte{0x00, 0x5f},
				Last:   []byte{0x00, 0x61},
				Values: [][]rune{[]rune("ff"), []rune("fi"), []rune("ffl")},
			},
		}) {
		t.Errorf("unexpected ranges: %v", parent.Ranges)
	}
	if parent.Parent != nil {
		t.Fatal("parent.Parent is not nil")
	}
}

func TestToUnicodeTemplate(t *testing.T) {
	buf := &bytes.Buffer{}

	// check that the template renders without error
	err := toUnicodeTmplNew.Execute(buf, testToUniInfoChild)
	if err != nil {
		t.Fatal(err)
	}

	// check that some key lines are present in the output
	body := buf.String()
	lines := []string{
		pdf.Format(testToUniInfoParent.MakeName()) + " usecmap",
		"/CMapName " + pdf.Format(testToUniInfoChild.MakeName()) + " def",
		"3 begincodespacerange\n<00> <fe>\n<ff00> <fffe>\n<ffff00> <ffffff>\nendcodespacerange",
		"3 begincidchar\n<02> <0041>\n<04> <0043>\n<05> <65e5>\nendcidchar",
		"2 begincidrange\n<07> <09> [<0047> <0048> <0049>]\n<ff10> <ff20> <00580041>\nendcidrange",
	}
	for _, line := range lines {
		if !strings.Contains(body, line) {
			t.Errorf("expected line %q not found in output", line)
		}
	}
}
