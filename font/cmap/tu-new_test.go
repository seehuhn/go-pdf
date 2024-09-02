package cmap

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

var _ pdf.Embedder[pdf.Unused] = (*ToUnicodeInfo)(nil)

var (
	parent = &ToUnicodeInfo{
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
	toUnicodeTest = &ToUnicodeInfo{
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
		Parent: parent,
	}
)

func TestMakeName(t *testing.T) {
	name1 := parent.MakeName()
	name2 := toUnicodeTest.MakeName()

	namePat := regexp.MustCompile(`^GoPDF-[0-9a-f]{32}-UTF16$`)
	for _, name := range []pdf.Name{name1, name2} {
		if !namePat.MatchString(string(name)) {
			t.Errorf("invalid name: %q", name)
		}
	}

	if name1 == name2 {
		t.Errorf("name1 and name2 should be different")
	}

	fmt.Println(name1)
	fmt.Println(name2)
}

func TestTemplate(t *testing.T) {
	buf := &bytes.Buffer{}
	err := toUnicodeTmplNew.Execute(buf, toUnicodeTest)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())
}

func BenchmarkMakeName(b *testing.B) {
	for range b.N {
		toUnicodeTest.MakeName()
	}
}
