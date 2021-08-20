package builtin

import (
	"os"
	"testing"
)

func Test14Fonts(t *testing.T) {
	known, err := afmData.ReadDir("afm")
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != len(FontNames) {
		t.Error("wrong number of afm files:", len(known))
	}

	for _, fontName := range FontNames {
		afm, err := Afm(fontName)
		if err != nil {
			t.Error(err)
			continue
		}

		if afm.FontName != fontName {
			t.Errorf("wrong font name: %q != %q", afm.FontName, fontName)
		}
	}
}

func TestError(t *testing.T) {
	_, err := Afm("unknown font")
	if !os.IsNotExist(err) {
		t.Errorf("wrong error: %s", err)
	}
}
