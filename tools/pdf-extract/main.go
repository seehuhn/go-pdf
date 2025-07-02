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
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/pdfcopy"
	"seehuhn.de/go/pdf/reader"
	"seehuhn.de/go/pdf/tools/pdf-extract/sections"
)

// PageSet represents a set of pages with Y coordinate bounds
type PageSet struct {
	Pages map[int]PageBounds // map from page number (0-based) to bounds
}

// PageBounds represents the Y coordinate bounds for a page
type PageBounds struct {
	YMin float64 // bottom bound (smaller Y value)
	YMax float64 // top bound (larger Y value)
}

// Region represents a way to select/filter pages
type Region interface {
	Apply(doc pdf.Getter, pages *PageSet) (*PageSet, error)
	String() string
}

// PageRegion represents page-based selection (page 1, pages 1-3, etc.)
type PageRegion struct {
	Start int  // -1 means from beginning
	End   int  // -1 means to end
	Odd   bool // select only odd pages
	Even  bool // select only even pages
}

func (pr PageRegion) String() string {
	if pr.Odd {
		return "pages odd"
	}
	if pr.Even {
		return "pages even"
	}
	if pr.Start == -1 && pr.End == -1 {
		return "pages all"
	}
	if pr.Start == pr.End {
		return fmt.Sprintf("page %d", pr.Start+1) // convert to 1-based for display
	}
	if pr.Start == -1 {
		return fmt.Sprintf("pages -%d", pr.End+1)
	}
	if pr.End == -1 {
		return fmt.Sprintf("pages %d-", pr.Start+1)
	}
	return fmt.Sprintf("pages %d-%d", pr.Start+1, pr.End+1)
}

func (pr PageRegion) Apply(doc pdf.Getter, pages *PageSet) (*PageSet, error) {
	result := &PageSet{Pages: make(map[int]PageBounds)}

	for pageNo, bounds := range pages.Pages {
		include := false

		// check page number bounds
		if pr.Start != -1 && pageNo < pr.Start {
			continue
		}
		if pr.End != -1 && pageNo > pr.End {
			continue
		}

		// check odd/even
		if pr.Odd && (pageNo+1)%2 == 0 { // pageNo is 0-based, so add 1 for 1-based odd check
			continue
		}
		if pr.Even && (pageNo+1)%2 == 1 {
			continue
		}

		include = true

		if include {
			result.Pages[pageNo] = bounds
		}
	}

	return result, nil
}

// SectionRegion represents section-based selection
type SectionRegion struct {
	Pattern string
}

func (sr SectionRegion) String() string {
	return fmt.Sprintf("section %q", sr.Pattern)
}

func (sr SectionRegion) Apply(doc pdf.Getter, pages *PageSet) (*PageSet, error) {
	// get the section page range
	sectionRange, err := sections.Pages(doc, sr.Pattern)
	if err != nil {
		return nil, fmt.Errorf("section selection failed: %w", err)
	}

	result := &PageSet{Pages: make(map[int]PageBounds)}

	for pageNo, bounds := range pages.Pages {
		if pageNo < sectionRange.FirstPage || pageNo > sectionRange.LastPage {
			continue
		}

		// adjust Y bounds based on section bounds
		newBounds := bounds
		if pageNo == sectionRange.FirstPage && sectionRange.YMax > 0 {
			// on first page, section goes from YMax downward - set upper bound
			if math.IsInf(bounds.YMax, +1) || bounds.YMax > sectionRange.YMax {
				newBounds.YMax = sectionRange.YMax
			}
		}
		if pageNo == sectionRange.LastPage && sectionRange.YMin > 0 {
			// on last page, section goes down to YMin - set lower bound
			if math.IsInf(bounds.YMin, -1) || bounds.YMin < sectionRange.YMin {
				newBounds.YMin = sectionRange.YMin
			}
		}

		result.Pages[pageNo] = newBounds
	}

	return result, nil
}

// OutputProcessor defines the interface for processing selected pages
type OutputProcessor interface {
	Process(doc pdf.Getter, pages *PageSet, outputFile string) error
	Name() string
}

// PDFExtractor extracts full pages to a new PDF file
type PDFExtractor struct{}

func (pe PDFExtractor) Name() string {
	return "PDF page extraction"
}

func (pe PDFExtractor) Process(doc pdf.Getter, pages *PageSet, outputFile string) error {
	if len(pages.Pages) == 0 {
		return fmt.Errorf("no pages selected for extraction")
	}

	// collect page numbers and sort them
	var pageNums []int
	for pageNo := range pages.Pages {
		pageNums = append(pageNums, pageNo)
	}

	// simple sort
	for i := 0; i < len(pageNums); i++ {
		for j := i + 1; j < len(pageNums); j++ {
			if pageNums[i] > pageNums[j] {
				pageNums[i], pageNums[j] = pageNums[j], pageNums[i]
			}
		}
	}

	// create output file
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// get input metadata
	metaIn := doc.GetMeta()

	// create output PDF writer
	out, err := pdf.NewWriter(outFile, metaIn.Version, nil)
	if err != nil {
		return fmt.Errorf("failed to create PDF writer: %w", err)
	}

	// create page tree writer and copier
	pageTreeOut := pagetree.NewWriter(out)
	copier := pdfcopy.NewCopier(out, doc)

	// extract each selected page
	for _, pageNo := range pageNums {
		// get the page from input
		refIn, pageIn, err := pagetree.GetPage(doc, pageNo)
		if err != nil {
			return fmt.Errorf("failed to get page %d: %w", pageNo+1, err)
		}

		// remove annotations to avoid reference issues (like in the example)
		delete(pageIn, "Annots")

		// copy the page dictionary
		pageOut, err := copier.CopyDict(pageIn)
		if err != nil {
			return fmt.Errorf("failed to copy page %d: %w", pageNo+1, err)
		}

		// allocate reference in output and redirect
		refOut := out.Alloc()
		if refIn != 0 {
			copier.Redirect(refIn, refOut)
		}

		// add page to output page tree
		pageTreeOut.AppendPageRef(refOut, pageOut)
	}

	// finalize page tree
	treeRef, err := pageTreeOut.Close()
	if err != nil {
		return fmt.Errorf("failed to close page tree: %w", err)
	}

	// set up output metadata
	metaOut := out.GetMeta()
	metaOut.Catalog.Pages = treeRef
	metaOut.Info = metaIn.Info

	// close output PDF
	err = out.Close()
	if err != nil {
		return fmt.Errorf("failed to close output PDF: %w", err)
	}

	fmt.Printf("Extracted %d pages to %s\n", len(pageNums), outputFile)
	return nil
}

// TextExtractor extracts text content (placeholder)
type TextExtractor struct{}

func (te TextExtractor) Name() string {
	return "text extraction"
}

func (te TextExtractor) Process(doc pdf.Getter, pages *PageSet, outputFile string) error {
	if len(pages.Pages) == 0 {
		return fmt.Errorf("no pages selected for text extraction")
	}

	// collect page numbers and sort them
	var pageNums []int
	for pageNo := range pages.Pages {
		pageNums = append(pageNums, pageNo)
	}

	// simple sort
	for i := 0; i < len(pageNums); i++ {
		for j := i + 1; j < len(pageNums); j++ {
			if pageNums[i] > pageNums[j] {
				pageNums[i], pageNums[j] = pageNums[j], pageNums[i]
			}
		}
	}

	// create output file
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// set up text extraction
	extraTextCache := make(map[font.Embedded]map[cid.CID]string)
	spaceWidth := make(map[font.Embedded]float64)

	contents := reader.New(doc, nil)
	contents.TextEvent = func(op reader.TextEvent, arg float64) {
		switch op {
		case reader.TextEventSpace:
			w0, ok := spaceWidth[contents.TextFont]
			if !ok {
				w0 = getSpaceWidth(contents.TextFont)
				spaceWidth[contents.TextFont] = w0
			}

			if arg > 0.3*w0 {
				fmt.Fprint(outFile, " ")
			}
		case reader.TextEventNL:
			fmt.Fprintln(outFile)
		case reader.TextEventMove:
			fmt.Fprintln(outFile)
		}
	}
	contents.Character = func(cid cid.CID, text string) error {
		if text == "" {
			F := contents.TextFont
			m, ok := extraTextCache[F]
			if !ok {
				m = getExtraMapping(doc, contents.TextFont)
				extraTextCache[F] = m
			}
			text = m[cid]
		}

		// for now, extract all text (YMin/YMax filtering will be added later)
		fmt.Fprint(outFile, text)
		return nil
	}

	// extract text from each selected page
	for _, pageNo := range pageNums {
		_, pageDict, err := pagetree.GetPage(doc, pageNo)
		if err != nil {
			return fmt.Errorf("failed to get page %d: %w", pageNo+1, err)
		}

		// add page separator if multiple pages
		if len(pageNums) > 1 {
			fmt.Fprintf(outFile, "\n--- Page %d ---\n\n", pageNo+1)
		}

		err = contents.ParsePage(pageDict, matrix.Identity)
		if err != nil {
			return fmt.Errorf("failed to parse page %d: %w", pageNo+1, err)
		}

		fmt.Fprintln(outFile)
	}

	fmt.Printf("Extracted text from %d pages to %s\n", len(pageNums), outputFile)
	return nil
}

// Helper functions for text extraction (adapted from examples/pdf-extract-text)

func getSpaceWidth(F font.Embedded) float64 {
	Fe, ok := F.(font.FromFile)
	if !ok {
		return 280
	}

	d := Fe.GetDict()
	if d == nil {
		return 0
	}

	return spaceWidthHeuristic(d)
}

func getExtraMapping(r pdf.Getter, F font.Embedded) map[cid.CID]string {
	Fe, ok := F.(font.FromFile)
	if !ok {
		return nil
	}

	d := Fe.GetDict()
	fontInfo := d.FontInfo()

	switch fontInfo := fontInfo.(type) {
	case *dict.FontInfoGlyfEmbedded:
		body, err := pdf.GetStreamReader(r, fontInfo.Ref)
		if err != nil {
			return nil
		}
		info, err := sfnt.Read(body)
		if err != nil {
			return nil
		}
		outlines, ok := info.Outlines.(*glyf.Outlines)
		if !ok {
			return nil
		}

		m := make(map[cid.CID]string)

		// method 1: use glyph names, if present
		if outlines.Names != nil {
			if fontInfo.CIDToGID != nil {
				for cidVal, gid := range fontInfo.CIDToGID {
					if int(gid) > len(outlines.Names) {
						continue
					}
					name := outlines.Names[gid]
					if name == "" {
						continue
					}

					text := names.ToUnicode(name, fontInfo.PostScriptName)
					m[cid.CID(cidVal)] = text
				}
			}
		}
		return m
	default:
		return nil
	}
}

type affine struct {
	intercept, slope float64
}

var commonCharacters = map[string]affine{
	" ": {0, 1},
	")": {-43.01937, 1.0268},
	"/": {-10.99708, 0.9623335},
	"•": {-24.2725, 0.9956384},
	"−": {-439.6255, 1.238626},
	"∗": {91.30598, 0.7265824},
	"1": {-130.7855, 0.9746186},
	"a": {-131.2164, 0.9740258},
	"A": {72.40703, 0.4928694},
	"e": {-136.5258, 0.9895894},
	"E": {-28.76257, 0.6957778},
	"i": {51.62929, 0.8973944},
	"ε": {-56.25771, 0.9947787},
	"Ω": {-132.9966, 1.002173},
	"中": {-356.8609, 1.215483},
}

func spaceWidthHeuristic(dict font.Dict) float64 {
	guesses := []float64{280}
	for _, info := range dict.Characters() {
		if coef, ok := commonCharacters[info.Text]; ok && info.Width > 0 {
			guesses = append(guesses, coef.intercept+coef.slope*info.Width)
		}
	}
	slices.Sort(guesses)

	// calculate the median
	var guess float64
	n := len(guesses)
	if n%2 == 0 {
		guess = (guesses[n/2-1] + guesses[n/2]) / 2
	} else {
		guess = guesses[n/2]
	}

	// adjustment to remove empirical bias
	guess = 1.366239*guess - 139.183703

	// clamp to approximate [0.01, 0.99] quantile range
	if guess < 200 {
		guess = 200
	} else if guess > 1000 {
		guess = 1000
	}

	return guess
}

// getOutputProcessor returns the appropriate processor based on file extension
func getOutputProcessor(filename string) (OutputProcessor, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return PDFExtractor{}, nil
	case ".txt":
		return TextExtractor{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s (supported: .pdf, .txt)", ext)
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: pdf-extract FILENAME [region ...] [to OUTPUT_FILE]")
	}

	filename := os.Args[1]
	args := os.Args[2:]

	// find "to" keyword to separate regions from output file
	var regionArgs []string
	var outputFile string

	toIndex := -1
	for i, arg := range args {
		if arg == "to" {
			toIndex = i
			break
		}
	}

	if toIndex >= 0 {
		regionArgs = args[:toIndex]
		if toIndex+1 >= len(args) {
			log.Fatal("expected output filename after 'to'")
		}
		outputFile = args[toIndex+1]
		if toIndex+2 < len(args) {
			log.Fatal("unexpected arguments after output filename")
		}
	} else {
		regionArgs = args
	}

	// open PDF
	doc, err := pdf.Open(filename, nil)
	if err != nil {
		log.Fatalf("failed to open PDF: %v", err)
	}
	defer doc.Close()

	// initialize with all pages
	totalPages := 0
	for range pagetree.NewIterator(doc).All() {
		totalPages++
	}

	initialPages := &PageSet{Pages: make(map[int]PageBounds)}
	for i := 0; i < totalPages; i++ {
		initialPages.Pages[i] = PageBounds{YMin: math.Inf(-1), YMax: math.Inf(+1)} // infinite bounds means full page
	}

	// parse and apply regions
	regions, err := parseRegions(regionArgs)
	if err != nil {
		log.Fatalf("failed to parse regions: %v", err)
	}

	currentPages := initialPages
	for _, region := range regions {
		currentPages, err = region.Apply(doc, currentPages)
		if err != nil {
			log.Fatalf("failed to apply region %s: %v", region, err)
		}
	}

	// handle output
	if outputFile != "" {
		processor, err := getOutputProcessor(outputFile)
		if err != nil {
			log.Fatalf("output error: %v", err)
		}

		err = processor.Process(doc, currentPages, outputFile)
		if err != nil {
			log.Fatalf("failed to process output: %v", err)
		}
	} else {
		// no output file specified, just print the result
		printPageSet(currentPages, regions)
	}
}

func parseRegions(args []string) ([]Region, error) {
	var regions []Region

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "page", "pages":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("expected page specification after %q", args[i])
			}
			i++
			region, err := parsePageRegion(args[i])
			if err != nil {
				return nil, fmt.Errorf("invalid page specification %q: %w", args[i], err)
			}
			regions = append(regions, region)

		case "section":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("expected section pattern after %q", args[i])
			}
			i++
			regions = append(regions, SectionRegion{Pattern: args[i]})

		default:
			return nil, fmt.Errorf("unknown region type %q", args[i])
		}
	}

	return regions, nil
}

func parsePageRegion(spec string) (PageRegion, error) {
	spec = strings.TrimSpace(spec)

	// handle special cases
	switch spec {
	case "odd":
		return PageRegion{Start: -1, End: -1, Odd: true}, nil
	case "even":
		return PageRegion{Start: -1, End: -1, Even: true}, nil
	case "all":
		return PageRegion{Start: -1, End: -1}, nil
	}

	// handle ranges
	if strings.Contains(spec, "-") {
		parts := strings.Split(spec, "-")
		if len(parts) != 2 {
			return PageRegion{}, fmt.Errorf("invalid range format")
		}

		var start, end int = -1, -1
		var err error

		if parts[0] != "" {
			start, err = strconv.Atoi(parts[0])
			if err != nil {
				return PageRegion{}, fmt.Errorf("invalid start page: %w", err)
			}
			start-- // convert to 0-based
		}

		if parts[1] != "" {
			end, err = strconv.Atoi(parts[1])
			if err != nil {
				return PageRegion{}, fmt.Errorf("invalid end page: %w", err)
			}
			end-- // convert to 0-based
		}

		return PageRegion{Start: start, End: end}, nil
	}

	// handle single page
	pageNum, err := strconv.Atoi(spec)
	if err != nil {
		return PageRegion{}, fmt.Errorf("invalid page number: %w", err)
	}
	pageNum-- // convert to 0-based

	return PageRegion{Start: pageNum, End: pageNum}, nil
}

func printPageSet(pages *PageSet, regions []Region) {
	if len(pages.Pages) == 0 {
		fmt.Println("No pages selected")
		return
	}

	// print applied regions
	if len(regions) > 0 {
		fmt.Print("Applied regions: ")
		for i, region := range regions {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(region)
		}
		fmt.Println()
	}

	// collect and sort page numbers
	var pageNums []int
	for pageNo := range pages.Pages {
		pageNums = append(pageNums, pageNo)
	}

	// simple sort
	for i := 0; i < len(pageNums); i++ {
		for j := i + 1; j < len(pageNums); j++ {
			if pageNums[i] > pageNums[j] {
				pageNums[i], pageNums[j] = pageNums[j], pageNums[i]
			}
		}
	}

	fmt.Printf("Selected pages (%d total):\n", len(pageNums))
	for _, pageNo := range pageNums {
		bounds := pages.Pages[pageNo]
		if math.IsInf(bounds.YMin, -1) && math.IsInf(bounds.YMax, +1) {
			fmt.Printf("  Page %d: full page\n", pageNo+1)
		} else {
			yMinStr := fmt.Sprintf("%g", bounds.YMin)
			if math.IsInf(bounds.YMin, -1) {
				yMinStr = "-∞"
			}
			yMaxStr := fmt.Sprintf("%g", bounds.YMax)
			if math.IsInf(bounds.YMax, +1) {
				yMaxStr = "+∞"
			}
			fmt.Printf("  Page %d: Y=%s to Y=%s\n", pageNo+1, yMinStr, yMaxStr)
		}
	}
}
