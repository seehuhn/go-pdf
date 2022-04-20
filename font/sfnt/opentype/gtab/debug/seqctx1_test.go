package debug

import (
	"os"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
)

func TestSequenceContext1(t *testing.T) {
	fontInfo := debug.MakeFont()

	fontInfo.Gdef = &gdef.Table{
		GlyphClass: classdef.Table{
			2: gdef.GlyphClassLigature,
			4: gdef.GlyphClassLigature,
		},
	}
	// fontInfo.Gpos = &gtab.Info{
	// 	ScriptList: map[gtab.ScriptLang]*gtab.Features{
	// 		{}: {}, // Required: 0
	// 	},
	// 	FeatureList: []*gtab.Feature{
	// 		{Tag: "test", Lookups: []gtab.LookupIndex{0}},
	// 	},
	// 	LookupList: []*gtab.LookupTable{
	// 		{
	// 			Meta: &gtab.LookupMetaInfo{
	// 				LookupType: 1,
	// 			},
	// 			Subtables: []gtab.Subtable{
	// 				&gtab.Gpos1_1{
	// 					Cov:    coverage.Table{2: 0, 4: 1},
	// 					Adjust: &gtab.ValueRecord{
	// 						// XAdvance: 1000,
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }
	fontInfo.Gsub = &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: []*gtab.LookupTable{
			{ // lookup 0
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
					LookupFlag: gtab.LookupIgnoreLigatures,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: map[font.GlyphID]int{1: 0},
						Rules: [][]gtab.SequenceRule{
							{
								{
									In: []font.GlyphID{3, 5},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 1, LookupListIndex: 1},
										{SequenceIndex: 1, LookupListIndex: 3},
										// {SequenceIndex: 1, LookupListIndex: 3},
										// {SequenceIndex: 2, LookupListIndex: 3},
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1
				Meta: &gtab.LookupMetaInfo{
					LookupType: 4,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub4_1{
						Cov: map[font.GlyphID]int{3: 0},
						Repl: [][]gtab.Ligature{
							{
								{
									In:  []font.GlyphID{4},
									Out: 9,
								},
							},
						},
					},
				},
			},
			{ // lookup 2
				Meta: &gtab.LookupMetaInfo{
					LookupType: 2,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub2_1{
						Cov:  map[font.GlyphID]int{3: 0},
						Repl: [][]font.GlyphID{{3, 4}},
					},
				},
			},
			{ // lookup 3
				Meta: &gtab.LookupMetaInfo{
					LookupType: 1,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_1{
						Cov:   map[font.GlyphID]int{1: 0, 2: 1, 3: 2, 4: 3, 5: 4, 9: 5},
						Delta: 20,
					},
				},
			},
		},
	}

	fd, err := os.Create("test.otf")
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

func Test9735(t *testing.T) {
	fontInfo := debug.MakeFont()

	fontInfo.Gdef = &gdef.Table{
		GlyphClass: classdef.Table{
			2: gdef.GlyphClassLigature,
			4: gdef.GlyphClassLigature,
		},
	}
	fontInfo.Gsub = &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: []*gtab.LookupTable{
			{ // lookup 0
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
					LookupFlag: gtab.LookupIgnoreLigatures,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: map[font.GlyphID]int{1: 0},
						Rules: [][]gtab.SequenceRule{
							{
								{
									In: []font.GlyphID{3, 5},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 1, LookupListIndex: 1},
										{SequenceIndex: 2, LookupListIndex: 2},
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1
				Meta: &gtab.LookupMetaInfo{
					LookupType: 4,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub4_1{
						Cov: map[font.GlyphID]int{3: 0},
						Repl: [][]gtab.Ligature{
							{
								{
									In:  []font.GlyphID{4},
									Out: 9,
								},
							},
						},
					},
				},
			},
			{ // lookup 2
				Meta: &gtab.LookupMetaInfo{
					LookupType: 1,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_1{
						Cov:   map[font.GlyphID]int{3: 0, 5: 1, 9: 2},
						Delta: 10,
					},
				},
			},
		},
	}

	fd, err := os.Create("test9735.otf")
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
