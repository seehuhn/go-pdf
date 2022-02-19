package cmap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFormat12Samples(t *testing.T) {
	// TODO(voss): remove
	names, err := filepath.Glob("../../../demo/try-all-fonts/cmap/12-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 {
		t.Fatal("not enough samples")
	}
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		_, err = decodeFormat12(data)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func FuzzFormat12(f *testing.F) {
	f.Add(format12{
		{startCharCode: 10, endCharCode: 20, startGlyphID: 30},
		{startCharCode: 1000, endCharCode: 2000, startGlyphID: 41},
		{startCharCode: 2000, endCharCode: 3000, startGlyphID: 1},
	}.Encode(0))

	f.Fuzz(func(t *testing.T, data []byte) {
		c1, err := decodeFormat12(data)
		if err != nil {
			return
		}

		data2 := c1.Encode(0)
		if len(data2) > len(data) {
			t.Error("too long")
		}

		c2, err := decodeFormat12(data2)
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(c1, c2) {
			t.Error("not equal")
		}
	})
}

var _ Subtable = format12(nil)
