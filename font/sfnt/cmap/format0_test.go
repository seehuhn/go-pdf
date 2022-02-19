package cmap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormat0Samples(t *testing.T) {
	// TODO(voss): remove
	names, err := filepath.Glob("../../../demo/try-all-fonts/cmap/00-*.bin")
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
		_, err = decodeFormat0(data)
		if err != nil {
			t.Fatal(err)
		}
	}
}

var _ Subtable = (*format0)(nil)
