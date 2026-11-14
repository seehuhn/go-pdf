// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
	"seehuhn.de/go/pdf/viewer-tests/internal/notdefgrid"
)

// The PostScript names of the two test fonts, and the files they are
// written to.  Both fonts must be installed for the viewer.
const (
	presentName = "Test28411P"
	missingName = "Test28411M"
	presentFile = "notdef-present.otf"
	missingFile = "notdef-missing.otf"
)

func main() {
	for _, f := range []struct {
		name     string
		fileName string
		present  bool
	}{
		{presentName, presentFile, true},
		{missingName, missingFile, false},
	} {
		err := writeFont(f.fileName, makeGlyphs(f.name, f.present))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}

	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const margin = 50.0

// codeTest is the code shown on the page.  The CMap maps it to CID 4, which
// only one of the two fonts contains, and gives CID 1 as its notdef CID.
const codeTest = 'E'

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A5r, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	noteFont := font.Must(standard.TimesRoman.New())
	note := text.F{
		Font:  noteFont,
		Size:  10,
		Color: color.DeviceGray(0.1),
	}
	label := text.F{
		Font:  noteFont,
		Size:  8,
		Color: color.DeviceGray(0.5),
	}

	var y float64

	text.Show(page.Builder,
		text.M{X: margin, Y: 360},
		note,
		text.Wrap(480,
			"Does an external font change the position of the next glyph?",
			"Install "+presentFile+" and "+missingFile+".",
			"Both rows show the same code with the same widths,",
			"but only the first font contains the glyph.",
			"The CMap gives a notdef CID for the code."),
		text.RecordPos{UserX: nil, UserY: &y},
	)

	y -= 8
	notdefgrid.Headers(page, label, margin, &y)
	for _, t := range []struct {
		name    string
		present bool
	}{
		{presentName, true},
		{missingName, false},
	} {
		test := text.F{
			Font:  makeTestFont(t.name),
			Size:  notdefgrid.FontSize,
			Color: color.DeviceRGB{0, 0, 0.8},
		}
		gIdx, wIdx := notdefgrid.ShowsNotdef, notdefgrid.Unspecified
		if t.present {
			gIdx, wIdx = notdefgrid.ShowsGlyph, notdefgrid.UsesGlyphWidth
		}
		rowLabel(page, label, y, t.name)
		notdefgrid.Row(page, test, margin, &y, codeTest, gIdx, wIdx)
	}

	text.Show(page.Builder,
		text.M{X: margin, Y: y - 14},
		note,
		text.Wrap(480,
			"Left: the glyph shown.  Right: the width used.",
			"A row without crosses means that the font was not loaded.",
			"If the crosses on the right are in different columns,",
			"the installed font has changed the layout."),
	)

	return page.Close()
}

// rowLabel shows a description to the right of the row with baseline y.
func rowLabel(page *document.Page, label text.F, y float64, s string) {
	text.Show(page.Builder,
		text.M{X: margin + notdefgrid.RowWidth + 8, Y: y + 2},
		label, s,
	)
}

// fontCreated is the fixed creation time of the test fonts, so that repeated
// runs give identical files.
var fontCreated = time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)

// makeGlyphs returns a CID-keyed font with the given PostScript name.  The
// font contains glyphs for the CIDs 0 to 3, and also for CID 4 if present
// is set.
func makeGlyphs(name string, present bool) *sfnt.Font {
	glyphs := []*cff.Glyph{
		notdefgrid.CrossGlyph(250, notdefgrid.WidthCID0),   // CID 0
		notdefgrid.CrossGlyph(750, notdefgrid.WidthNotdef), // CID 1
		notdefgrid.CrossGlyph(1250, notdefgrid.WidthGlyph), // CID 2
		notdefgrid.CrossGlyph(-1, 500),                     // CID 3, blank
	}
	gidToCID := []cid.CID{0, 1, 2, 3}
	if present {
		glyphs = append(glyphs,
			notdefgrid.CrossGlyph(1250, notdefgrid.WidthGlyph)) // CID 4
		gidToCID = append(gidToCID, 4)
	}

	F := &sfnt.Font{
		FontName:   name,
		FamilyName: name,
		Subfamily:  "Regular",
		FullName:   name,
		Weight:     os2.WeightNormal,
		Width:      os2.WidthNormal,
		IsRegular:  true,

		Version:          0x00010000,
		CreationTime:     fontCreated,
		ModificationTime: fontCreated,
		PermUse:          os2.PermInstall,

		UnitsPerEm: 1000,
		Ascent:     800,
		Descent:    -200,
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},

		Outlines: &cff.Outlines{
			Glyphs: glyphs,
			Private: []*type1.PrivateDict{
				{
					BlueValues: []float64{-10, 0, 290, 300},
					BlueScale:  0.039625,
					BlueShift:  7,
					BlueFuzz:   1,
					StdHW:      20,
					StdVW:      20,
				},
			},
			FDSelect: func(glyph.ID) int {
				return 0
			},
			ROS:          ros,
			GIDToCID:     gidToCID,
			FontMatrices: []matrix.Matrix{matrix.Identity},
		},
	}

	// a Unicode cmap table, so that font managers accept the file
	table := sfntcmap.Format4{}
	for gid, c := range gidToCID {
		if gid > 0 {
			table[uint16('A')+uint16(c)] = glyph.ID(gid)
		}
	}
	F.InstallCMap(table)
	F.CodePageRange.Set(os2.CP1252)

	return F
}

// ros is the character collection of all test fonts.  The identity
// collection is the one meant for fonts with their own glyph set, where a
// viewer has no basis for replacing the font by a different one.
var ros = &cid.SystemInfo{
	Registry:   "Adobe",
	Ordering:   "Identity",
	Supplement: 0,
}

func writeFont(filename string, F *sfnt.Font) error {
	fd, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, err = F.Write(fd)
	if err != nil {
		fd.Close()
		return err
	}
	return fd.Close()
}

// makeTestFont returns the PDF font for the external font with the given
// name.  The fonts made by this function differ only in the name.
func makeTestFont(name string) font.Instance {
	enc := &cmap.File{
		Name:  "Test28411-H",
		ROS:   ros,
		WMode: font.Horizontal,
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{'A'}, High: []byte{'Z'}},
		},
		CIDRanges: []cmap.Range{
			{First: []byte{'A'}, Last: []byte{'Z'}, Value: 0},
		},
		NotdefSingles: []cmap.Single{
			{Code: []byte{codeTest}, Value: 1},
		},
	}

	widths := map[cid.CID]float64{
		0: notdefgrid.WidthCID0,
		1: notdefgrid.WidthNotdef,
		2: notdefgrid.WidthGlyph,
		3: 500,
		4: notdefgrid.WidthGlyph, // the glyph which may be missing
	}

	fd := &font.Descriptor{
		FontName:   name,
		IsSymbolic: true,
		FontBBox:   rect.Rect{LLx: 0, LLy: 0, URx: 1500, URy: 300},
		Ascent:     300,
		Descent:    0,
		Leading:    400,
		StemV:      0,
	}
	res := &dict.CIDFontType0{
		PostScriptName:  name,
		Descriptor:      fd,
		ROS:             ros,
		CMap:            enc,
		Width:           widths,
		DefaultWidth:    notdefgrid.WidthDefault,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
	}
	return res.MakeFont()
}
