package cmap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font"
)

func TestFormat6Samples(t *testing.T) {
	// TODO(voss): remove
	names, err := filepath.Glob("../../../demo/try-all-fonts/cmap/06-*.bin")
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
		_, err = decodeFormat6(data)
		if err != nil {
			t.Errorf("failed to decode %q: %s", name, err)
			continue
		}
	}
}

func FuzzFormat6(f *testing.F) {
	f.Add((&format6{
		FirstCode:    123,
		GlyphIDArray: []font.GlyphID{6, 4, 2},
	}).Encode(0))

	f.Fuzz(func(t *testing.T, data []byte) {
		c1, err := decodeFormat6(data)
		if err != nil {
			return
		}

		data2 := c1.Encode(0)

		c2, err := decodeFormat6(data2)
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(c1, c2) {
			fmt.Printf("A: % x\n", data)
			fmt.Printf("B: % x\n", data2)
			t.Error("not equal")
		}
	})
}

var _ Subtable = (*format6)(nil)
