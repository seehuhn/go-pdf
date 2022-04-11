package debug

import (
	"os"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/locale"
)

func TestGsub(t *testing.T) {
	gsub := &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{Script: locale.ScriptLatin}: {
				Required: 0xFFFF,
				Optional: []gtab.FeatureIndex{0},
			},
		},
		FeatureList: []*gtab.Feature{
			{Tag: "ccmp", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: []*gtab.LookupTable{
			{
				Meta: &gtab.LookupMetaInfo{},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_1{
						Cov:   map[font.GlyphID]int{3: 0},
						Delta: 26,
					},
				},
			},
		},
	}
	trfm := gsub.GetTransformation(locale.EnUS, map[string]bool{"ccmp": true})

	unpack := func(gg []font.Glyph) []font.GlyphID {
		res := make([]font.GlyphID, len(gg))
		for i, g := range gg {
			res[i] = g.Gid
		}
		return res
	}

	in := []font.Glyph{
		{Gid: 1},
		{Gid: 2},
		{Gid: 3},
		{Gid: 4},
		{Gid: 5},
	}
	expected := []font.GlyphID{1, 2, 29, 4, 5}
	gg := trfm.Apply(in)
	if out := unpack(gg); !reflect.DeepEqual(out, expected) {
		t.Errorf("expected %v, got %v", expected, out)
	}

	fontInfo, err := debug.MakeFont()
	if err != nil {
		t.Fatal(err)
	}
	fontInfo.Gsub = gsub
	fd, err := os.Create("000.otf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fontInfo.Write(fd)
	if err != nil {
		t.Fatal(err)
	}
	err = fd.Close()
	if err != nil {
		t.Error(err)
	}
}
