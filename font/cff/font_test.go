package cff

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMany(t *testing.T) {
	names, err := filepath.Glob("../../demo/try-all-fonts/cff/*.cff")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		fd, err := os.Open(name)
		if err != nil {
			t.Error(err)
			continue
		}
		_, err = Read(fd)
		if err != nil {
			fd.Close()
			t.Error(err)
			continue
		}

		// ...

		err = fd.Close()
		if err != nil {
			t.Error(err)
		}
	}
}
