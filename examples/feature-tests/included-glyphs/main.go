// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"os"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/extended"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/outline"
	"seehuhn.de/go/sfnt/glyph"
)

const numRows = 23

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	paper := document.A4
	doc, err := document.CreateMultiPage(fname, paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	doc.Out.GetMeta().Info = &pdf.Info{
		Title:   "Quire Font Library - Glyph Reference",
		Subject: "Display of all glyphs in extended and Go fonts",
		Creator: "seehuhn.de/go/pdf/examples/feature-tests/included-glyphs",
	}

	headerFont := standard.HelveticaBold.New()
	bodyFont := standard.Helvetica.New()

	// Create DocumentWriter
	writer := &DocumentWriter{
		doc:           doc,
		headerFont:    headerFont,
		bodyFont:      bodyFont,
		currentPageNo: 0,
		outline:       &outline.Tree{},
	}

	// Track pages for table of contents
	var allFonts []fontEntry

	// Extended fonts
	for _, f := range extended.All {
		fontInstance := f.New()
		name := getFontDisplayName(f)
		usage := getExtendedFontUsage(f)

		allFonts = append(allFonts, fontEntry{
			name:         name,
			usageExample: usage,
			fontInstance: fontInstance,
		})
	}

	for _, f := range gofont.All {
		fontInstance, err := f.New(nil)
		if err != nil {
			return fmt.Errorf("failed to create Go font %v: %w", f, err)
		}
		name := getGoFontDisplayName(f)
		usage := getGoFontUsage(f)

		allFonts = append(allFonts, fontEntry{
			name:         name,
			usageExample: usage,
			fontInstance: fontInstance,
		})
	}

	// Create title page
	err = writer.createTitlePage(len(allFonts))
	if err != nil {
		return err
	}

	// Create table of contents page (without page numbers since they're dynamic)
	err = writer.createTableOfContents(allFonts)
	if err != nil {
		return err
	}

	// Create font pages with multi-page subsetting for fonts with >256 non-blank glyphs
	for _, entry := range allFonts {
		err = writer.createFontPages(entry)
		if err != nil {
			return err
		}
	}

	// Write outline
	writer.outline.Write(doc.Out)

	return doc.Close()
}

type fontEntry struct {
	name         string
	usageExample string
	fontInstance font.Layouter
}

// DocumentWriter holds information required to write text to pages
type DocumentWriter struct {
	doc           *document.MultiPage
	headerFont    font.Layouter
	bodyFont      font.Layouter
	currentPageNo int // 0-based
	outline       *outline.Tree
}

func (w *DocumentWriter) createTitlePage(fontCount int) error {
	page := w.doc.AddPage()
	paper := document.A4

	page.TextBegin()
	page.TextSetFont(w.headerFont, 24)
	page.TextFirstLine(paper.XPos(0.5), paper.YPos(0.7))
	page.TextShowAligned("Quire Font Library", 0, 0.5)

	page.TextSetFont(w.bodyFont, 16)
	page.TextFirstLine(0, -40)
	page.TextShowAligned("Glyph Reference", 0, 0.5)

	page.TextSetFont(w.bodyFont, 12)
	page.TextFirstLine(0, -60)
	page.TextShowAligned(fmt.Sprintf("Displaying the %d included fonts.", fontCount), 0, 0.5)
	page.TextEnd()

	// Add page number - moved down slightly
	page.TextBegin()
	page.TextSetFont(w.bodyFont, 10)
	page.TextFirstLine(paper.XPos(0.5), 30)
	page.TextShowAligned(fmt.Sprintf("%d", w.currentPageNo+1), 0, 0.5)
	page.TextEnd()

	w.currentPageNo++
	return page.Close()
}

func (w *DocumentWriter) createTableOfContents(fonts []fontEntry) error {
	page := w.doc.AddPage()
	paper := document.A4

	// Title
	page.TextBegin()
	page.TextSetFont(w.headerFont, 18)
	page.TextFirstLine(72, paper.URy-72)
	page.TextShow("Table of Contents")
	page.TextEnd()

	// Font entries
	yPos := paper.URy - 120
	lineHeight := 16.0

	for _, fontEntry := range fonts {
		if yPos < 100 {
			break
		}

		// Draw the font name
		page.TextBegin()
		page.TextSetFont(w.bodyFont, 12)
		page.TextFirstLine(72, yPos)
		page.TextShow(fontEntry.name)
		page.TextEnd()

		yPos -= lineHeight
	}

	// Add page number - moved down slightly
	page.TextBegin()
	page.TextSetFont(w.bodyFont, 10)
	page.TextFirstLine(paper.XPos(0.5), 30)
	page.TextShowAligned(fmt.Sprintf("%d", w.currentPageNo+1), 0, 0.5)
	page.TextEnd()

	w.currentPageNo++
	return page.Close()
}

// getNonBlankGlyphs returns all glyphs that have non-zero extents (visible glyphs)
func getNonBlankGlyphs(fontInstance font.Layouter) []glyph.ID {
	geom := fontInstance.GetGeometry()
	var nonBlankGlyphs []glyph.ID

	for i := 1; i < len(geom.GlyphExtents); i++ {
		gid := glyph.ID(i)
		if geom.GlyphExtents[gid].IsZero() {
			continue
		}
		nonBlankGlyphs = append(nonBlankGlyphs, gid)
	}

	return nonBlankGlyphs
}

func (w *DocumentWriter) createFontPages(entry fontEntry) error {
	// Add to outline
	oFont := w.outline.AddChild(entry.name)

	// Get all non-blank glyphs from this font
	nonBlankGlyphs := getNonBlankGlyphs(entry.fontInstance)

	if len(nonBlankGlyphs) == 0 {
		// Skip fonts with no displayable glyphs
		return nil
	}

	// Calculate glyphs per page: 16 columns × 22 rows = 352 slots
	const glyphsPerPageCapacity = numRows * 16
	totalPages := (len(nonBlankGlyphs) + glyphsPerPageCapacity - 1) / glyphsPerPageCapacity

	// Set outline destination to first page of this font (current page number)
	dest := pdf.Array{
		pdf.Integer(w.currentPageNo),
		pdf.Name("XYZ"),
		pdf.Integer(0),
		pdf.Number(document.A4.URy),
		pdf.Integer(0),
	}
	oFont.Action = pdf.Dict{
		"S": pdf.Name("GoTo"),
		"D": dest,
	}

	// Create pages for each chunk
	for pageIndex := 0; pageIndex < totalPages; pageIndex++ {
		startIdx := pageIndex * glyphsPerPageCapacity
		endIdx := startIdx + glyphsPerPageCapacity
		if endIdx > len(nonBlankGlyphs) {
			endIdx = len(nonBlankGlyphs)
		}

		pageGlyphs := nonBlankGlyphs[startIdx:endIdx]

		err := w.createFontPageWithMultipleInstances(entry, pageIndex, pageGlyphs)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *DocumentWriter) createFontPageWithMultipleInstances(entry fontEntry, pageIndex int, pageGlyphs []glyph.ID) error {
	page := w.doc.AddPage()
	paper := document.A4

	// Font name header (only on first page)
	if pageIndex == 0 {
		page.TextBegin()
		page.TextSetFont(w.headerFont, 16)
		page.TextFirstLine(72, paper.URy-67)
		page.TextShow(entry.name)
		page.TextEnd()
	}

	// Usage example (only on first page) - moved closer to title
	if pageIndex == 0 {
		page.TextBegin()
		page.TextSetFont(w.bodyFont, 10)
		page.TextFirstLine(72, paper.URy-83)
		page.TextShow("Usage: " + entry.usageExample)
		page.TextEnd()
	}

	// Split page glyphs into font instance chunks (max 256 per instance)
	const maxGlyphsPerFontInstance = 256
	currentRow := 0

	for i := 0; i < len(pageGlyphs); i += maxGlyphsPerFontInstance {
		endIdx := i + maxGlyphsPerFontInstance
		if endIdx > len(pageGlyphs) {
			endIdx = len(pageGlyphs)
		}

		instanceGlyphs := pageGlyphs[i:endIdx]

		// Create fresh font instance for this chunk
		instanceFont, err := createFreshFontInstance(entry)
		if err != nil {
			return err
		}

		// Draw this font instance chunk, continuing from current row
		// Start lower to give more space between usage line and glyphs
		newRow, err := w.drawGlyphChunkFromRow(page, instanceFont, 72, paper.URy-125, instanceGlyphs, currentRow)
		if err != nil {
			return err
		}
		currentRow = newRow
	}

	// Add page number - moved down slightly
	page.TextBegin()
	page.TextSetFont(w.bodyFont, 10)
	page.TextFirstLine(paper.XPos(0.5), 30)
	page.TextShowAligned(fmt.Sprintf("%d", w.currentPageNo+1), 0, 0.5)
	page.TextEnd()

	w.currentPageNo++
	return page.Close()
}

func createFreshFontInstance(entry fontEntry) (font.Layouter, error) {
	// Determine if this is an extended font or Go font and create a fresh instance
	fontName := entry.name

	// Check if it's a Go font (composite)
	if strings.Contains(fontName, "Go ") {
		// Find the matching gofont and create new instance
		for _, f := range gofont.All {
			if getGoFontDisplayName(f) == fontName {
				return f.New(&truetype.Options{
					Composite: true,
				})
			}
		}
	}

	// Check if it's an extended font
	for _, f := range extended.All {
		if getFontDisplayName(f) == fontName {
			return f.New(), nil
		}
	}

	return nil, fmt.Errorf("unknown font: %s", fontName)
}

func (w *DocumentWriter) drawGlyphChunkFromRow(page *document.Page, fontInstance font.Layouter, startX, startY float64, glyphChunk []glyph.ID, startRow int) (int, error) {
	const glyphSize = 20
	const glyphsPerRow = 16
	const cellWidth = 30
	const cellHeight = 30

	geom := fontInstance.GetGeometry()

	// Get Unicode mapping for copy-paste support
	unicodeMap := getUnicodeMapping(fontInstance)
	reverseUnicodeMap := make(map[rune]glyph.ID)
	for gid, r := range unicodeMap {
		reverseUnicodeMap[r] = gid
	}

	row := startRow
	col := 0

	// For each glyph in chunk, try to create it by laying out its Unicode character
	// This ensures proper font subsetting
	for _, originalGID := range glyphChunk {
		x := startX + float64(col)*cellWidth
		y := startY - float64(row)*cellHeight

		// Stop if we exceed the number of rows
		if row >= numRows {
			break
		}

		// Also stop if we run out of page space (backup check)
		if y < 50 {
			break
		}

		var gg *font.GlyphSeq

		// Try to use Unicode character if available
		if unicode, exists := unicodeMap[originalGID]; exists {
			// Use Layout to create the glyph sequence from Unicode
			gg = fontInstance.Layout(nil, glyphSize, string(unicode))

			// Make sure we got the right glyph
			if gg != nil && len(gg.Seq) > 0 && gg.Seq[0].GID == originalGID {
				// Perfect match - use as is
			} else {
				// Fallback - create glyph manually
				gg = &font.GlyphSeq{
					Seq: []font.Glyph{{
						GID:     originalGID,
						Advance: glyphSize * geom.Widths[originalGID],
						Text:    string(unicode),
					}},
				}
			}
		} else {
			// No Unicode mapping - create glyph manually
			gg = &font.GlyphSeq{
				Seq: []font.Glyph{{
					GID:     originalGID,
					Advance: glyphSize * geom.Widths[originalGID],
				}},
			}
		}

		if gg != nil {
			gg.Align(0, 0.5)

			page.TextBegin()
			page.TextSetFont(fontInstance, glyphSize)
			page.TextFirstLine(x+cellWidth/2, y)
			page.TextShowGlyphs(gg)
			page.TextEnd()
		}

		col++
		if col >= glyphsPerRow {
			col = 0
			row++
		}
	}

	// Return the final row position (if we ended mid-row, advance to next row)
	if col > 0 {
		row++
	}
	return row, nil
}

func getUnicodeMapping(fontInstance font.Layouter) map[glyph.ID]rune {
	result := make(map[glyph.ID]rune)

	// Create a simple mapping by laying out characters and seeing what glyph IDs we get
	for r := rune(32); r <= rune(126); r++ { // ASCII printable characters
		seq := fontInstance.Layout(nil, 12, string(r))
		if seq != nil && len(seq.Seq) > 0 {
			gid := seq.Seq[0].GID
			if _, exists := result[gid]; !exists {
				result[gid] = r
			}
		}
	}

	// Add some common extended characters including Go gopher symbol
	extendedChars := []rune{
		0x00A0, // non-breaking space
		0x00A1, // inverted exclamation mark
		0x00A2, // cent sign
		0x00A3, // pound sign
		0x00A4, // currency sign
		0x00A5, // yen sign
		0x00C0, // À
		0x00C1, // Á
		0x00E0, // à
		0x00E1, // á
		0xF800, // Go gopher symbol
	}

	for _, r := range extendedChars {
		seq := fontInstance.Layout(nil, 12, string(r))
		if seq != nil && len(seq.Seq) > 0 {
			gid := seq.Seq[0].GID
			if _, exists := result[gid]; !exists {
				result[gid] = r
			}
		}
	}

	return result
}

func getFontDisplayName(f extended.Font) string {
	names := map[extended.Font]string{
		extended.D050000L:               "D050000L (ZapfDingbats Extended)",
		extended.NimbusMonoPSBold:       "NimbusMonoPS-Bold (Courier-Bold Extended)",
		extended.NimbusMonoPSBoldItalic: "NimbusMonoPS-BoldItalic (Courier-BoldOblique Extended)",
		extended.NimbusMonoPSItalic:     "NimbusMonoPS-Italic (Courier-Oblique Extended)",
		extended.NimbusMonoPSRegular:    "NimbusMonoPS-Regular (Courier Extended)",
		extended.NimbusRomanBold:        "NimbusRoman-Bold (Times-Bold Extended)",
		extended.NimbusRomanBoldItalic:  "NimbusRoman-BoldItalic (Times-BoldItalic Extended)",
		extended.NimbusRomanItalic:      "NimbusRoman-Italic (Times-Italic Extended)",
		extended.NimbusRomanRegular:     "NimbusRoman-Regular (Times-Roman Extended)",
		extended.NimbusSansBold:         "NimbusSans-Bold (Helvetica-Bold Extended)",
		extended.NimbusSansBoldItalic:   "NimbusSans-BoldItalic (Helvetica-BoldOblique Extended)",
		extended.NimbusSansItalic:       "NimbusSans-Italic (Helvetica-Oblique Extended)",
		extended.NimbusSansRegular:      "NimbusSans-Regular (Helvetica Extended)",
		extended.StandardSymbolsPS:      "StandardSymbolsPS (Symbol Extended)",
	}
	return names[f]
}

func getExtendedFontUsage(f extended.Font) string {
	names := map[extended.Font]string{
		extended.D050000L:               "extended.D050000L.New()",
		extended.NimbusMonoPSBold:       "extended.NimbusMonoPSBold.New()",
		extended.NimbusMonoPSBoldItalic: "extended.NimbusMonoPSBoldItalic.New()",
		extended.NimbusMonoPSItalic:     "extended.NimbusMonoPSItalic.New()",
		extended.NimbusMonoPSRegular:    "extended.NimbusMonoPSRegular.New()",
		extended.NimbusRomanBold:        "extended.NimbusRomanBold.New()",
		extended.NimbusRomanBoldItalic:  "extended.NimbusRomanBoldItalic.New()",
		extended.NimbusRomanItalic:      "extended.NimbusRomanItalic.New()",
		extended.NimbusRomanRegular:     "extended.NimbusRomanRegular.New()",
		extended.NimbusSansBold:         "extended.NimbusSansBold.New()",
		extended.NimbusSansBoldItalic:   "extended.NimbusSansBoldItalic.New()",
		extended.NimbusSansItalic:       "extended.NimbusSansItalic.New()",
		extended.NimbusSansRegular:      "extended.NimbusSansRegular.New()",
		extended.StandardSymbolsPS:      "extended.StandardSymbolsPS.New()",
	}
	return names[f]
}

func getGoFontDisplayName(f gofont.Font) string {
	names := map[gofont.Font]string{
		gofont.Regular:         "Go Regular",
		gofont.Bold:            "Go Bold",
		gofont.BoldItalic:      "Go Bold Italic",
		gofont.Italic:          "Go Italic",
		gofont.Medium:          "Go Medium",
		gofont.MediumItalic:    "Go Medium Italic",
		gofont.Smallcaps:       "Go Smallcaps",
		gofont.SmallcapsItalic: "Go Smallcaps Italic",
		gofont.Mono:            "Go Mono",
		gofont.MonoBold:        "Go Mono Bold",
		gofont.MonoBoldItalic:  "Go Mono Bold Italic",
		gofont.MonoItalic:      "Go Mono Italic",
	}
	return names[f]
}

func getGoFontUsage(f gofont.Font) string {
	names := map[gofont.Font]string{
		gofont.Regular:         "gofont.Regular.New(nil)",
		gofont.Bold:            "gofont.Bold.New(nil)",
		gofont.BoldItalic:      "gofont.BoldItalic.New(nil)",
		gofont.Italic:          "gofont.Italic.New(nil)",
		gofont.Medium:          "gofont.Medium.New(nil)",
		gofont.MediumItalic:    "gofont.MediumItalic.New(nil)",
		gofont.Smallcaps:       "gofont.Smallcaps.New(nil)",
		gofont.SmallcapsItalic: "gofont.SmallcapsItalic.New(nil)",
		gofont.Mono:            "gofont.Mono.New(nil)",
		gofont.MonoBold:        "gofont.MonoBold.New(nil)",
		gofont.MonoBoldItalic:  "gofont.MonoBoldItalic.New(nil)",
		gofont.MonoItalic:      "gofont.MonoItalic.New(nil)",
	}
	return names[f]
}
