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
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"

	"seehuhn.de/go/pdf/tools/internal/buildinfo"
	"seehuhn.de/go/pdf/tools/internal/profile"
	"seehuhn.de/go/pdf/tools/pdf-extract/sections"
	"seehuhn.de/go/pdf/tools/pdf-extract/text"
)

// config holds all command-line flag values.
type config struct {
	outputType      string
	force           bool
	showNextSection bool
	noActualText    bool
	showPageNumbers bool
}

// PageSet represents a set of pages with coordinate bounds.
type PageSet struct {
	Pages map[int]PageBounds // map from page number (0-based) to bounds
}

// SortedPages returns the page numbers in ascending order.
func (ps *PageSet) SortedPages() []int {
	pages := make([]int, 0, len(ps.Pages))
	for pageNo := range ps.Pages {
		pages = append(pages, pageNo)
	}
	slices.Sort(pages)
	return pages
}

// PageBounds represents the coordinate bounds for a page.
type PageBounds struct {
	XMin float64 // left bound
	XMax float64 // right bound
	YMin float64 // bottom bound (smaller Y value)
	YMax float64 // top bound (larger Y value)
}

// Region represents a way to select/filter pages.
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

func (pr PageRegion) Apply(_ pdf.Getter, pages *PageSet) (*PageSet, error) {
	result := &PageSet{Pages: make(map[int]PageBounds)}

	for pageNo, bounds := range pages.Pages {
		if pr.Start != -1 && pageNo < pr.Start {
			continue
		}
		if pr.End != -1 && pageNo > pr.End {
			continue
		}
		if pr.Odd && (pageNo+1)%2 == 0 { // pageNo is 0-based, so add 1 for 1-based odd check
			continue
		}
		if pr.Even && (pageNo+1)%2 == 1 {
			continue
		}
		result.Pages[pageNo] = bounds
	}

	return result, nil
}

// SectionRegion represents section-based selection.
type SectionRegion struct {
	Pattern string
}

func (sr SectionRegion) String() string {
	return fmt.Sprintf("section %q", sr.Pattern)
}

func (sr SectionRegion) Apply(doc pdf.Getter, pages *PageSet) (*PageSet, error) {
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
		if pageNo == sectionRange.FirstPage && !math.IsInf(sectionRange.YMax, +1) {
			// on first page, section goes from YMax downward - set upper bound
			if math.IsInf(bounds.YMax, +1) || bounds.YMax > sectionRange.YMax {
				newBounds.YMax = sectionRange.YMax
			}
		}
		if pageNo == sectionRange.LastPage && !math.IsInf(sectionRange.YMin, -1) {
			// on last page, section goes down to YMin - set lower bound
			if math.IsInf(bounds.YMin, -1) || bounds.YMin < sectionRange.YMin {
				newBounds.YMin = sectionRange.YMin
			}
		}

		result.Pages[pageNo] = newBounds
	}

	return result, nil
}

// XRangeRegion represents X coordinate range filtering.
type XRangeRegion struct {
	Min float64
	Max float64
}

func (xr XRangeRegion) String() string {
	return fmt.Sprintf("xrange %g-%g", xr.Min, xr.Max)
}

func (xr XRangeRegion) Apply(_ pdf.Getter, pages *PageSet) (*PageSet, error) {
	result := &PageSet{Pages: make(map[int]PageBounds)}
	for pageNo, bounds := range pages.Pages {
		newBounds := bounds
		if newBounds.XMin < xr.Min {
			newBounds.XMin = xr.Min
		}
		if newBounds.XMax > xr.Max {
			newBounds.XMax = xr.Max
		}
		result.Pages[pageNo] = newBounds
	}
	return result, nil
}

// OutputProcessor defines the interface for processing selected pages.
type OutputProcessor interface {
	Process(doc pdf.Getter, pages *PageSet, w io.Writer) error
}

// PDFExtractor extracts full pages to a new PDF file.
type PDFExtractor struct{}

func (pe PDFExtractor) Process(doc pdf.Getter, pages *PageSet, w io.Writer) error {
	if len(pages.Pages) == 0 {
		return fmt.Errorf("no pages selected for extraction")
	}

	pageNums := pages.SortedPages()

	// get input metadata
	metaIn := doc.GetMeta()

	// create output PDF writer
	out, err := pdf.NewWriter(w, metaIn.Version, nil)
	if err != nil {
		return fmt.Errorf("failed to create PDF writer: %w", err)
	}

	// create page tree writer and copier
	rm := pdf.NewResourceManager(out)
	pageTreeOut := pagetree.NewWriter(out, rm)
	copier := pdf.NewCopier(out, doc)

	// extract each selected page
	for _, pageNo := range pageNums {
		refIn, pageIn, err := pagetree.GetPage(doc, pageNo)
		if err != nil {
			return fmt.Errorf("failed to get page %d: %w", pageNo+1, err)
		}

		// remove annotations to avoid reference issues
		delete(pageIn, "Annots")

		pageOut, err := copier.CopyDict(pageIn)
		if err != nil {
			return fmt.Errorf("failed to copy page %d: %w", pageNo+1, err)
		}

		refOut := out.Alloc()
		if refIn != 0 {
			copier.Redirect(refIn, refOut)
		}

		if err := pageTreeOut.AppendPageDict(refOut, pageOut); err != nil {
			return fmt.Errorf("failed to append page %d: %w", pageNo+1, err)
		}
	}

	// finalize page tree
	treeRef, err := pageTreeOut.Close()
	if err != nil {
		return fmt.Errorf("failed to close page tree: %w", err)
	}

	err = rm.Close()
	if err != nil {
		return fmt.Errorf("failed to close resource manager: %w", err)
	}

	// set up output metadata
	metaOut := out.GetMeta()
	metaOut.Catalog.Pages = treeRef
	metaOut.Info = metaIn.Info

	return out.Close()
}

// TextExtractor extracts text content.
type TextExtractor struct {
	UseActualText   bool
	ShowPageNumbers bool
}

func (te TextExtractor) Process(doc pdf.Getter, pages *PageSet, w io.Writer) error {
	if len(pages.Pages) == 0 {
		return fmt.Errorf("no pages selected for text extraction")
	}

	pageNums := pages.SortedPages()

	// create text extractor
	extractor := text.New(doc, w)
	extractor.UseActualText = te.UseActualText

	// extract text from each selected page
	for _, pageNo := range pageNums {
		bounds := pages.Pages[pageNo]
		extractor.XRangeMin = bounds.XMin
		extractor.XRangeMax = bounds.XMax

		_, pageDict, err := pagetree.GetPage(doc, pageNo)
		if err != nil {
			return fmt.Errorf("failed to get page %d: %w", pageNo+1, err)
		}

		if te.ShowPageNumbers {
			fmt.Fprintf(w, "--- Page %d ---\n\n", pageNo+1)
		}

		err = extractor.ExtractPage(pageDict)
		if err != nil {
			return fmt.Errorf("failed to extract page %d: %w", pageNo+1, err)
		}

		fmt.Fprintln(w)
	}

	return nil
}

// getOutputProcessor returns the appropriate processor based on file extension
// or explicit type.
func getOutputProcessor(filename, explicitType string, cfg config) (OutputProcessor, error) {
	var fileType string
	if explicitType != "" {
		fileType = explicitType
	} else if filename == "-" {
		fileType = "pdf"
	} else {
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".pdf":
			fileType = "pdf"
		case ".txt":
			fileType = "txt"
		default:
			return nil, fmt.Errorf("unsupported output format: %s (supported: .pdf, .txt)", ext)
		}
	}

	switch fileType {
	case "pdf":
		return PDFExtractor{}, nil
	case "txt":
		return TextExtractor{
			UseActualText:   !cfg.noActualText,
			ShowPageNumbers: cfg.showPageNumbers,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported file type: %s (supported: pdf, txt)", fileType)
	}
}

// openOutputFile opens the output file for writing. If outputFile is "-",
// os.Stdout is returned. Otherwise, the file is opened with overwrite
// protection unless forceOverwrite is set.
func openOutputFile(outputFile string, forceOverwrite bool) (io.Writer, io.Closer, error) {
	if outputFile == "-" {
		return os.Stdout, nil, nil
	}

	flags := os.O_WRONLY | os.O_CREATE
	if forceOverwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(outputFile, flags, 0666)
	if err != nil {
		if os.IsExist(err) {
			return nil, nil, fmt.Errorf("file %s already exists (use -f to overwrite)", outputFile)
		}
		return nil, nil, err
	}

	return file, file, nil
}

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")

	var cfg config
	flag.StringVar(&cfg.outputType, "type", "", "output type (pdf or txt), overrides file extension")
	flag.BoolVar(&cfg.force, "f", false, "overwrite output file if it exists")
	flag.BoolVar(&cfg.showNextSection, "show-next-section", false, "show the name of the next section after processing")
	flag.BoolVar(&cfg.noActualText, "no-actualtext", false, "disable ActualText substitution")
	flag.BoolVar(&cfg.showPageNumbers, "P", false, "show page numbers in text output")
	help := flag.Bool("help", false, "show help information")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "pdf-extract \u2014 extract pages or text from a PDF file\n")
		fmt.Fprintf(os.Stderr, "%s\n\n", buildinfo.Short("pdf-extract"))
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  pdf-extract [options] <file.pdf> [region...] [to <output>]\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  file.pdf   PDF file to extract from\n")
		fmt.Fprintf(os.Stderr, "  output     output file (.pdf or .txt), or - for stdout\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nRegion types:\n")
		fmt.Fprintf(os.Stderr, "  page N       extract page N (1-based)\n")
		fmt.Fprintf(os.Stderr, "  pages N-M    extract pages N through M\n")
		fmt.Fprintf(os.Stderr, "  pages odd    extract odd-numbered pages\n")
		fmt.Fprintf(os.Stderr, "  pages even   extract even-numbered pages\n")
		fmt.Fprintf(os.Stderr, "  section PAT  extract section matching regex pattern PAT\n")
		fmt.Fprintf(os.Stderr, "  xrange A-B   restrict to X coordinates A through B\n")
		fmt.Fprintf(os.Stderr, "\nQueries (no output file needed):\n")
		fmt.Fprintf(os.Stderr, "  sections     list all sections in document\n")
		fmt.Fprintf(os.Stderr, "  pages        show total page count\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  pdf-extract doc.pdf page 1 to page1.pdf\n")
		fmt.Fprintf(os.Stderr, "  pdf-extract doc.pdf section \"Intro\" to intro.txt\n")
		fmt.Fprintf(os.Stderr, "  pdf-extract doc.pdf page 1 to -\n")
		fmt.Fprintf(os.Stderr, "  pdf-extract -type txt doc.pdf section \"Intro\" xrange 100-500 to -\n")
		fmt.Fprintf(os.Stderr, "  pdf-extract doc.pdf sections\n")
		fmt.Fprintf(os.Stderr, "  pdf-extract doc.pdf pages\n")
	}

	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(cfg, *cpuprofile, *memprofile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cfg config, cpuprofile, memprofile string) error {
	stop, err := profile.Start(cpuprofile, memprofile)
	if err != nil {
		return err
	}
	defer stop()

	args := flag.Args()
	filename := args[0]
	regionArgs := args[1:]

	// find "to" keyword to separate regions from output file
	var processedRegionArgs []string
	var outputFile string

	toIndex := -1
	for i, arg := range regionArgs {
		if arg == "to" {
			toIndex = i
			break
		}
	}

	if toIndex >= 0 {
		processedRegionArgs = regionArgs[:toIndex]
		if toIndex+1 >= len(regionArgs) {
			return fmt.Errorf("expected output filename after 'to'")
		}
		outputFile = regionArgs[toIndex+1]
		if toIndex+2 < len(regionArgs) {
			return fmt.Errorf("unexpected arguments after output filename")
		}
	} else {
		processedRegionArgs = regionArgs
	}

	// open PDF
	doc, err := pdf.Open(filename, nil)
	if err != nil {
		return fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	// initialize with all pages
	totalPages, err := pagetree.NumPages(doc)
	if err != nil {
		return fmt.Errorf("failed to read page tree: %w", err)
	}

	initialPages := &PageSet{Pages: make(map[int]PageBounds)}
	for i := range totalPages {
		initialPages.Pages[i] = PageBounds{
			XMin: math.Inf(-1),
			XMax: math.Inf(+1),
			YMin: math.Inf(-1),
			YMax: math.Inf(+1),
		}
	}

	// check for special cases that don't require full processing
	if len(processedRegionArgs) == 1 {
		switch processedRegionArgs[0] {
		case "section", "sections":
			ss, err := sections.ListAll(doc)
			if err != nil {
				return fmt.Errorf("failed to list sections: %w", err)
			}
			if len(ss) == 0 {
				fmt.Println("No sections found in document")
			} else {
				fmt.Println("Sections:")
				for _, section := range ss {
					fmt.Println(section)
				}
			}
			return nil
		case "page", "pages":
			fmt.Printf("Total pages: %d\n", totalPages)
			return nil
		}
	}

	// parse and apply regions
	regions, err := parseRegions(processedRegionArgs)
	if err != nil {
		return fmt.Errorf("failed to parse regions: %w", err)
	}

	// check if we need to track the next section for -show-next-section flag
	var nextSectionTitle string
	if cfg.showNextSection {
		var sectionPattern string
		for _, region := range regions {
			if sr, ok := region.(SectionRegion); ok {
				sectionPattern = sr.Pattern
				break
			}
		}
		if sectionPattern == "" {
			return fmt.Errorf("-show-next-section can only be used with section-based selection")
		}
		nextSectionTitle, err = sections.FindNext(doc, sectionPattern)
		if err != nil {
			return fmt.Errorf("failed to find next section: %w", err)
		}
	}

	currentPages := initialPages
	for _, region := range regions {
		currentPages, err = region.Apply(doc, currentPages)
		if err != nil {
			return fmt.Errorf("failed to apply region %s: %w", region, err)
		}
	}

	// handle output
	if outputFile != "" {
		processor, err := getOutputProcessor(outputFile, cfg.outputType, cfg)
		if err != nil {
			return err
		}

		w, closer, err := openOutputFile(outputFile, cfg.force)
		if err != nil {
			return err
		}

		err = processor.Process(doc, currentPages, w)
		if err != nil {
			if closer != nil {
				closer.Close()
				os.Remove(outputFile)
			}
			return err
		}

		if closer != nil {
			err = closer.Close()
			if err != nil {
				return err
			}
		}

		outputName := outputFile
		if outputName == "-" {
			outputName = "stdout"
		}
		fmt.Fprintf(os.Stderr, "extracted %d pages to %s\n", len(currentPages.Pages), outputName)
	} else {
		// no output file specified, just print the result
		printPageSet(currentPages, regions)
	}

	// show next section if requested
	if cfg.showNextSection && nextSectionTitle != "" {
		fmt.Println(nextSectionTitle)
	}

	return nil
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

		case "section", "sections":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("expected section pattern after %q", args[i])
			}
			i++
			regions = append(regions, SectionRegion{Pattern: args[i]})

		case "xrange":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("expected range specification after %q", args[i])
			}
			i++
			var xMin, xMax float64
			_, err := fmt.Sscanf(args[i], "%f-%f", &xMin, &xMax)
			if err != nil || xMin >= xMax {
				return nil, fmt.Errorf("invalid x-range specification %q (expected A-B where A < B)", args[i])
			}
			regions = append(regions, XRangeRegion{Min: xMin, Max: xMax})

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

	pageNums := pages.SortedPages()

	fmt.Printf("Selected pages (%d total):\n", len(pageNums))
	for _, pageNo := range pageNums {
		bounds := pages.Pages[pageNo]
		xFull := math.IsInf(bounds.XMin, -1) && math.IsInf(bounds.XMax, +1)
		yFull := math.IsInf(bounds.YMin, -1) && math.IsInf(bounds.YMax, +1)

		if xFull && yFull {
			fmt.Printf("  Page %d: full page\n", pageNo+1)
			continue
		}

		var parts []string
		if !yFull {
			yMinStr := fmt.Sprintf("%g", bounds.YMin)
			if math.IsInf(bounds.YMin, -1) {
				yMinStr = "-\u221e"
			}
			yMaxStr := fmt.Sprintf("%g", bounds.YMax)
			if math.IsInf(bounds.YMax, +1) {
				yMaxStr = "+\u221e"
			}
			parts = append(parts, fmt.Sprintf("Y=%s to %s", yMinStr, yMaxStr))
		}
		if !xFull {
			xMinStr := fmt.Sprintf("%g", bounds.XMin)
			if math.IsInf(bounds.XMin, -1) {
				xMinStr = "-\u221e"
			}
			xMaxStr := fmt.Sprintf("%g", bounds.XMax)
			if math.IsInf(bounds.XMax, +1) {
				xMaxStr = "+\u221e"
			}
			parts = append(parts, fmt.Sprintf("X=%s to %s", xMinStr, xMaxStr))
		}
		fmt.Printf("  Page %d: %s\n", pageNo+1, strings.Join(parts, ", "))
	}
}
