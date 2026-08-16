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

// Package pagelabel implements PDF page labels (PDF spec section 12.4.2).
//
// Page labels allow documents to define custom numbering for pages, such as
// Roman numerals for front matter or prefixed numbers for appendices.
// The document is divided into labelling ranges, each with its own numbering
// style, optional prefix, and start value.
package pagelabel

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/numtree"
)

// Style identifies the numbering style for the numeric portion of page labels.
type Style uint8

const (
	// None indicates no numeric portion; the label consists solely of the prefix.
	None Style = iota

	// Decimal uses Arabic numerals: 1, 2, 3, ...
	Decimal

	// UpperRoman uses uppercase Roman numerals: I, II, III, ...
	UpperRoman

	// LowerRoman uses lowercase Roman numerals: i, ii, iii, ...
	LowerRoman

	// UpperAlpha uses uppercase letters: A, B, ..., Z, AA, BB, ...
	UpperAlpha

	// LowerAlpha uses lowercase letters: a, b, ..., z, aa, bb, ...
	LowerAlpha
)

// styleToPDFName maps Style values to the corresponding PDF name objects.
var styleToPDFName = [...]pdf.Name{
	Decimal:    "D",
	UpperRoman: "R",
	LowerRoman: "r",
	UpperAlpha: "A",
	LowerAlpha: "a",
}

// pdfNameToStyle maps PDF name strings to Style values.
var pdfNameToStyle = map[pdf.Name]Style{
	"D": Decimal,
	"R": UpperRoman,
	"r": LowerRoman,
	"A": UpperAlpha,
	"a": LowerAlpha,
}

// Range describes the page labelling rules for a contiguous range of pages
// (PDF spec Table 161, "Entries in a page label dictionary").
type Range struct {
	// Style is the numbering style for the numeric portion of each page label.
	// If None, labels consist solely of the Prefix.
	Style Style

	// Prefix is the label prefix prepended to the numeric portion.
	Prefix string

	// Start is the value of the numeric portion for the first page in the
	// range.  Subsequent pages are numbered sequentially from this value.
	// Must be >= 1.  Default is 1.
	Start int
}

// Format returns the label string for a page at the given offset within this
// range (0 = first page of the range).
func (r *Range) Format(offset int) string {
	n := r.Start + offset
	var numeric string
	switch r.Style {
	case None:
		return r.Prefix
	case Decimal:
		numeric = formatDecimal(n)
	case UpperRoman:
		numeric = strings.ToUpper(formatRoman(n))
	case LowerRoman:
		numeric = formatRoman(n)
	case UpperAlpha:
		numeric = strings.ToUpper(formatAlpha(n))
	case LowerAlpha:
		numeric = formatAlpha(n)
	}
	return r.Prefix + numeric
}

// Entry is a labelling range together with the page where it starts.
type Entry struct {
	// FirstPage is the 0-based index of the first page the range covers.
	FirstPage int

	Range
}

// Labels holds page labelling information for a document.
type Labels struct {
	// Ranges lists the labelling ranges, sorted by FirstPage in strictly
	// ascending order.  The first entry covers page 0.
	Ranges []Entry
}

// New creates a Labels from a list of labelling ranges.
// The FirstPage values are 0-based page indices and must be in strictly
// ascending order.  The first entry must have FirstPage 0.
// A Start value below 1 is replaced by the default value 1.
func New(entries ...Entry) (*Labels, error) {
	l := &Labels{Ranges: slices.Clone(entries)}
	for i := range l.Ranges {
		if l.Ranges[i].Start < 1 {
			l.Ranges[i].Start = 1
		}
	}
	if err := l.validate(); err != nil {
		return nil, err
	}
	return l, nil
}

// validate checks the invariants of [Labels.Ranges].  Both [New] and
// [Labels.Embed] use it, so no invalid Labels can reach a PDF file.
// Everything [Extract] returns satisfies it.
func (l *Labels) validate() error {
	if len(l.Ranges) == 0 {
		return errors.New("no page label ranges")
	}
	if l.Ranges[0].FirstPage != 0 {
		return fmt.Errorf("first labelling range starts at page %d, not 0", l.Ranges[0].FirstPage)
	}
	for i, e := range l.Ranges {
		if i > 0 && e.FirstPage <= l.Ranges[i-1].FirstPage {
			return fmt.Errorf("labelling ranges out of order: %d after %d", e.FirstPage, l.Ranges[i-1].FirstPage)
		}
		if e.Start < 1 {
			return fmt.Errorf("labelling range at page %d has start value %d", e.FirstPage, e.Start)
		}
		if int(e.Style) >= len(styleToPDFName) {
			return fmt.Errorf("invalid numbering style %d at page %d", e.Style, e.FirstPage)
		}
	}
	return nil
}

// Extract reads page labels from a PDF PageLabels number tree object.
func Extract(r pdf.Getter, obj pdf.Object) (*Labels, error) {
	c := pdf.NewCursor(r)
	tree, err := numtree.ExtractInMemory(r, obj)
	if err != nil {
		return nil, err
	}
	if tree == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing PageLabels number tree"),
		}
	}

	l := &Labels{}
	for key, val := range tree.All() {
		// A key names the page index of the first page of a range, so it
		// cannot be negative.  Skip anything outside the range of a page
		// index, so that the ranges stay sorted and can be fed back into
		// [New].
		if key < 0 || key > math.MaxInt {
			continue
		}

		lr := Entry{
			FirstPage: int(key),
			Range: Range{
				Start: 1,
			},
		}

		// a value which is not a dictionary is treated like a missing one
		dict, err := pdf.Optional(c.Dict(val))
		if err != nil {
			return nil, err
		}
		if dict == nil {
			l.Ranges = append(l.Ranges, lr)
			continue
		}

		s, err := c.Name(dict["S"])
		if err == nil && s != "" {
			if style, ok := pdfNameToStyle[s]; ok {
				lr.Style = style
			}
		}

		p, err := c.TextString(dict["P"])
		if err == nil {
			lr.Prefix = string(p)
		}

		st, err := c.Integer(dict["St"])
		if err == nil && st >= 1 {
			lr.Start = int(min(st, limits.MaxPageLabelStart))
		}

		l.Ranges = append(l.Ranges, lr)
	}

	// ensure sorted by page index
	slices.SortFunc(l.Ranges, func(a, b Entry) int {
		return a.FirstPage - b.FirstPage
	})

	// insert a default range at page 0 if missing
	if len(l.Ranges) == 0 || l.Ranges[0].FirstPage != 0 {
		l.Ranges = slices.Insert(l.Ranges, 0, Entry{
			FirstPage: 0,
			Range:     Range{Style: Decimal, Start: 1},
		})
	}

	return l, nil
}

// Embed writes page labels as a number tree into a PDF file.
func (l *Labels) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := l.validate(); err != nil {
		return nil, err
	}

	opt := rm.Out().GetOptions()

	data := func(yield func(pdf.Integer, pdf.Object) bool) {
		for _, lr := range l.Ranges {
			dict := pdf.Dict{}
			if opt.HasAny(pdf.OptDictTypes) {
				dict["Type"] = pdf.Name("PageLabel")
			}
			if lr.Style != None {
				dict["S"] = styleToPDFName[lr.Style]
			}
			if lr.Prefix != "" {
				dict["P"] = pdf.TextString(lr.Prefix)
			}
			if lr.Start != 1 {
				dict["St"] = pdf.Integer(lr.Start)
			}
			if !yield(pdf.Integer(lr.FirstPage), dict) {
				return
			}
		}
	}

	return numtree.Write(rm.Out(), data)
}

// Format returns the label string for the given 0-based page index.
func (l *Labels) Format(pageIndex int) string {
	ri, offset := l.RangeAt(pageIndex)
	if ri < 0 {
		return formatDecimal(pageIndex + 1)
	}
	return l.Ranges[ri].Format(offset)
}

// RangeAt returns the range index and offset within that range for a
// 0-based page index.  Returns -1, 0 if no range covers the page.
func (l *Labels) RangeAt(pageIndex int) (rangeIndex, offset int) {
	if len(l.Ranges) == 0 {
		return -1, 0
	}

	// binary search for the last range whose FirstPage <= pageIndex
	i, _ := slices.BinarySearchFunc(l.Ranges, pageIndex, func(lr Entry, target int) int {
		return lr.FirstPage - target
	})
	// i is the insertion point; the range we want is i-1 if the exact
	// match was not found
	if i < len(l.Ranges) && l.Ranges[i].FirstPage == pageIndex {
		return i, 0
	}
	i--
	if i < 0 {
		return -1, 0
	}
	return i, pageIndex - l.Ranges[i].FirstPage
}
