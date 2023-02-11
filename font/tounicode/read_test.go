package tounicode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead(t *testing.T) {
	ff, err := filepath.Glob("../../../examples/try-all-pdfs/toUnicode/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range ff {
		fd, err := os.Open(f)
		if err != nil {
			t.Error(err)
			continue
		}

		_, err = Read(fd)
		if err != nil {
			t.Error(f, err)
		}

		fd.Close()
	}
}
