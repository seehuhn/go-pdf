package parser

import (
	"fmt"
	"path/filepath"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/locale"
)

func TestGtab(t *testing.T) {
	allTTF, err := filepath.Glob("../../truetype/ttf/*.ttf")
	if err != nil {
		t.Fatal(err)
	}
	allOTF, err := filepath.Glob("../../opentype/otf/*.otf")
	if err != nil {
		t.Fatal(err)
	}

	fonts := append(allTTF, allOTF...)
	for _, font := range fonts {
		sfnt, err := sfnt.Open(font)
		if err != nil {
			t.Error(err)
			continue
		}
		p := New(sfnt)

		gsub, err := p.ReadGsubTable(locale.EnGB)
		if err != nil {
			t.Error(err)
		}
		gpos, err := p.ReadGposTable(locale.EnGB)
		if err != nil {
			t.Error(err)
		}

		fmt.Println(font, len(gsub), len(gpos))
		for i, l := range gsub {
			for j, s := range l.subtables {
				fmt.Printf("\tGSUB %d.%d %T\n", i, j, s)
			}
		}
		for i, l := range gpos {
			for j, s := range l.subtables {
				fmt.Printf("\tGPOS %d.%d %T\n", i, j, s)
			}
		}

		err = sfnt.Close()
		if err != nil {
			t.Error(err)
		}
	}
}
