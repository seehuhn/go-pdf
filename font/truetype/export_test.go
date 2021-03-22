package truetype

import (
	"os"
	"testing"
)

func TestExport(t *testing.T) {
	tt, err := Open("RobotoSlab-Light.ttf")
	if err != nil {
		t.Fatal(err)
	}

	out, err := os.Create("out.ttf")
	if err != nil {
		t.Fatal(err)
	}

	err = tt.Export(out)
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
}
