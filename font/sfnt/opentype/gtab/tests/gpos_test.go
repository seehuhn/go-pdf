package tests

import (
	"fmt"
	"os"
	"testing"

	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
)

func TestGpos(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')

	lookupType := uint16(1)
	subtables := gtab.Subtables{
		&gtab.Gpos1_1{
			Cov:    coverage.Table{gidA: 0},
			Adjust: &gtab.ValueRecord{YPlacement: 500},
		},
	}

	gpos := &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: []*gtab.LookupTable{
			{
				Meta: &gtab.LookupMetaInfo{
					LookupType: lookupType,
				},
				Subtables: subtables,
			},
		},
	}
	fontInfo.Gpos = gpos

	testIdx := 1234
	fname := fmt.Sprintf("test%03d.otf", testIdx)
	fd, err := os.Create(fname)
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
