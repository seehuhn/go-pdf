package truetype

import (
	"os"
	"testing"
)

func TestExport(t *testing.T) {
	tt, err := Open("ttf/FreeSerif.ttf")
	if err != nil {
		t.Fatal(err)
	}

	out, err := os.Create("out.ttf")
	if err != nil {
		t.Fatal(err)
	}

	n, err := tt.export(out, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = tt.Close()
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat("out.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != n {
		t.Errorf("wrong size: %d != %d", fi.Size(), n)
	}
}
