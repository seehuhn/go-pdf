package parser

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
)

func TestInterpreter(t *testing.T) {
	tt, err := sfnt.Open("../../truetype/ttf/SourceSerif4-Regular.ttf")
	// tt, err := sfnt.Open("../../truetype/ttf/FreeSerif.ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer tt.Close()

	targetScript := "latn"
	targetLang := "ENG "
	tableName := "GSUB"

	includeFeature := make(map[string]bool)
	if tableName == "GSUB" {
		includeFeature["ccmp"] = true
		includeFeature["liga"] = true
		includeFeature["clig"] = true
	} else { // tableName == "GPOS"
		includeFeature["kern"] = true
		includeFeature["mark"] = true
		includeFeature["mkmk"] = true
	}

	p := New(tt)
	gtab, err := newGTab(p, targetScript, targetLang)
	if err != nil {
		t.Fatal(err)
	}
	err = gtab.init(tableName, includeFeature)
	if err != nil {
		t.Fatal(err)
	}

	if tableName == "GSUB" {
		for name, ii := range gtab.lookupIndices {
			fmt.Println("\n\n" + name)
			for _, i := range ii {
				_, err := gtab.getGsubLookup(i, "")
				if err != nil {
					t.Fatal(err)
				}
				fmt.Println()
			}
		}
	}

	t.Error("fish")
}
