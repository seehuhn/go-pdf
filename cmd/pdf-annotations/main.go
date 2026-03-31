// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	"os"
	"slices"
	"strings"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/cmd/internal/buildinfo"
	"seehuhn.de/go/pdf/pagetree"
)

var (
	passwdArg = flag.String("p", "", "PDF password")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "pdf-annotations — list markup annotations in a PDF file\n")
		fmt.Fprintf(os.Stderr, "%s\n\n", buildinfo.Short("pdf-annotations"))
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  pdf-annotations [options] <file.pdf>...\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  file.pdf   one or more PDF files to inspect\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	for _, fname := range flag.Args() {
		if err := processFile(fname); err != nil {
			return err
		}
	}
	return nil
}

// annotEntry holds a decoded markup annotation together with its PDF reference.
type annotEntry struct {
	ref    pdf.Reference
	annot  annotation.Annotation
	common *annotation.Common
	markup *annotation.Markup
}

func processFile(fname string) error {
	var opt *pdf.ReaderOptions
	if *passwdArg != "" {
		opt = &pdf.ReaderOptions{
			Password: *passwdArg,
		}
	}

	r, err := pdf.Open(fname, opt)
	if err != nil {
		return err
	}
	defer r.Close()

	numPages, err := pagetree.NumPages(r)
	if err != nil {
		return err
	}

	x := pdf.NewExtractor(r)
	first := true

	for pageNo := range numPages {
		_, pageDict, err := pagetree.GetPage(r, pageNo)
		if err != nil {
			return err
		}

		annotsArray, err := pdf.GetArray(r, pageDict["Annots"])
		if err != nil || annotsArray == nil {
			continue
		}

		// decode all annotations, keep only markup types
		var entries []annotEntry
		for _, obj := range annotsArray {
			ref, _ := obj.(pdf.Reference)

			a, err := annotation.Decode(x, nil, obj, false)
			if err != nil {
				continue
			}

			m := getMarkup(a)
			if m == nil {
				continue
			}

			entries = append(entries, annotEntry{
				ref:    ref,
				annot:  a,
				common: a.GetCommon(),
				markup: m,
			})
		}

		if len(entries) == 0 {
			continue
		}

		if !first {
			fmt.Println()
		}
		first = false
		fmt.Printf("Page %d:\n", pageNo+1)

		printAnnotationTree(entries)
	}

	return nil
}

// printAnnotationTree builds the reply tree and prints it.
func printAnnotationTree(entries []annotEntry) {
	// build index from reference to entry
	byRef := make(map[pdf.Reference]*annotEntry)
	for i := range entries {
		if entries[i].ref != 0 {
			byRef[entries[i].ref] = &entries[i]
		}
	}

	// build parent→children map
	children := make(map[pdf.Reference][]*annotEntry)
	var roots []*annotEntry
	for i := range entries {
		e := &entries[i]
		irt := e.markup.InReplyTo
		if irt != 0 {
			if _, ok := byRef[irt]; ok {
				children[irt] = append(children[irt], e)
				continue
			}
		}
		roots = append(roots, e)
	}

	// sort by creation date (dateless first)
	sortByDate(roots)
	for ref := range children {
		sortByDate(children[ref])
	}

	// print tree
	for i, root := range roots {
		if i > 0 {
			fmt.Println()
		}
		printEntry(root, "  ", "  ", children)
	}
}

func sortByDate(entries []*annotEntry) {
	slices.SortStableFunc(entries, func(a, b *annotEntry) int {
		aDate := a.markup.CreationDate
		bDate := b.markup.CreationDate
		aZero := aDate.IsZero()
		bZero := bDate.IsZero()
		if aZero != bZero {
			if aZero {
				return -1
			}
			return 1
		}
		return aDate.Compare(bDate)
	})
}

// printEntry prints a single annotation and its children.
// prefix is the indentation for this entry's first line.
// contPrefix is the indentation for continuation lines and children.
func printEntry(e *annotEntry, prefix, contPrefix string, children map[pdf.Reference][]*annotEntry) {
	typeName := string(e.annot.AnnotationType())
	rect := formatRect(e.common.Rect)

	rt := e.markup.RT
	var label string
	if rt == "Group" {
		label = "Group: "
	} else if e.markup.InReplyTo != 0 {
		label = "Reply: "
	}

	// line 1: type, contents summary, rect
	contents := e.common.Contents
	fmt.Printf("%s%s%s %s\n", prefix, label, typeName, rect)

	// line 2: date and user
	var meta []string
	if !e.markup.CreationDate.IsZero() {
		meta = append(meta, e.markup.CreationDate.Format(time.DateOnly))
	}
	if e.markup.User != "" {
		meta = append(meta, e.markup.User)
	}
	if e.markup.Subject != "" {
		meta = append(meta, fmt.Sprintf("[%s]", e.markup.Subject))
	}
	if len(meta) > 0 {
		fmt.Printf("%s  %s\n", contPrefix, strings.Join(meta, "  "))
	}

	// contents text, wrapped
	if contents != "" {
		printWrapped(contents, contPrefix+"  ", 78)
	}

	// print children
	kids := children[e.ref]
	for i, child := range kids {
		last := i == len(kids)-1
		var childPrefix, childCont string
		if last {
			childPrefix = contPrefix + "└─ "
			childCont = contPrefix + "   "
		} else {
			childPrefix = contPrefix + "├─ "
			childCont = contPrefix + "│  "
		}
		printEntry(child, childPrefix, childCont, children)
	}
}

func formatRect(r pdf.Rectangle) string {
	w := max(r.URx-r.LLx, r.LLx-r.URx)
	h := max(r.URy-r.LLy, r.LLy-r.URy)
	x := min(r.LLx, r.URx)
	y := min(r.LLy, r.URy)
	return fmt.Sprintf("(%.0fx%.0f+%.0f+%.0f)", w, h, x, y)
}

// printWrapped prints text word-wrapped at maxWidth, preserving paragraph
// breaks.  Per PDF spec 12.5.6.2, paragraphs in Contents are separated by
// carriage return (0x0D).  We also accept \n and \r\n.
func printWrapped(text, indent string, maxWidth int) {
	// normalize line endings to \n
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	paragraphs := strings.Split(text, "\n")
	width := maxWidth - len(indent)
	width = max(width, 20)

	for i, para := range paragraphs {
		if i > 0 {
			fmt.Printf("%s\n", indent)
		}
		if para == "" {
			continue
		}
		lines := wrapLine(para, width)
		for _, line := range lines {
			fmt.Printf("%s%s\n", indent, line)
		}
	}
}

// wrapLine breaks a single paragraph into lines of at most width characters.
func wrapLine(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	lines = append(lines, current)
	return lines
}

// getMarkup returns the Markup fields from a markup annotation, or nil if
// the annotation is not a markup type.
func getMarkup(a annotation.Annotation) *annotation.Markup {
	switch v := a.(type) {
	case *annotation.Text:
		return &v.Markup
	case *annotation.FreeText:
		return &v.Markup
	case *annotation.Line:
		return &v.Markup
	case *annotation.Square:
		return &v.Markup
	case *annotation.Circle:
		return &v.Markup
	case *annotation.Polygon:
		return &v.Markup
	case *annotation.PolyLine:
		return &v.Markup
	case *annotation.TextMarkup:
		return &v.Markup
	case *annotation.Caret:
		return &v.Markup
	case *annotation.Stamp:
		return &v.Markup
	case *annotation.Ink:
		return &v.Markup
	case *annotation.FileAttachment:
		return &v.Markup
	case *annotation.Sound:
		return &v.Markup
	case *annotation.Redact:
		return &v.Markup
	case *annotation.Projection:
		return &v.Markup
	default:
		return nil
	}
}
