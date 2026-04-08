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

package jbig2

import (
	"fmt"
	"sort"

	"seehuhn.de/go/pdf/graphics/bitmap"
	internaljbig2 "seehuhn.de/go/pdf/internal/filter/jbig2"
)

// Decode decodes a JBIG2 image from PDF globals and page streams.
// The globals stream may be nil if there are no global segments.
func Decode(globals, page []byte) (*bitmap.Bitmap, error) {
	return internaljbig2.Decode(globals, page)
}

// Encoder builds JBIG2 data for PDF embedding.
// Symbol dictionaries are shared across all encoded images via the globals
// stream.
type Encoder struct {
	// GenericTemplate selects the arithmetic coding template (0-3).
	GenericTemplate int

	symbols    []*bitmap.Bitmap
	nextSegNum uint32
	sdSegNum   uint32 // segment number of the SD in globals
	sdData     []byte // cached globals data
	idMap      []int  // idMap[original] = reordered index
}

// NewEncoder creates a new JBIG2 encoder.
func NewEncoder() *Encoder {
	return &Encoder{}
}

// AddSymbol adds a glyph bitmap to the shared symbol dictionary.
// Returns the symbol ID.
func (e *Encoder) AddSymbol(bm *bitmap.Bitmap) int {
	id := len(e.symbols)
	e.symbols = append(e.symbols, bm)
	e.sdData = nil // invalidate cached globals
	e.idMap = nil
	return id
}

// Globals returns the globals stream bytes containing the shared symbol
// dictionary. Call after all symbols have been added.
func (e *Encoder) Globals() ([]byte, error) {
	if e.sdData != nil {
		return e.sdData, nil
	}
	if len(e.symbols) == 0 {
		return nil, nil
	}

	// group symbols by height for height classes
	type symEntry struct {
		idx    int
		width  int
		height int
	}
	entries := make([]symEntry, len(e.symbols))
	for i, s := range e.symbols {
		entries[i] = symEntry{i, s.Width(), s.Height()}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].height != entries[j].height {
			return entries[i].height < entries[j].height
		}
		return entries[i].width < entries[j].width
	})

	// reorder symbols by height class and build old→new mapping
	reordered := make([]*bitmap.Bitmap, len(e.symbols))
	e.idMap = make([]int, len(e.symbols))
	for i, ent := range entries {
		reordered[i] = e.symbols[ent.idx]
		e.idMap[ent.idx] = i
	}

	sdData := internaljbig2.EncodeSymbolDictSegment(reordered, e.GenericTemplate)
	e.sdSegNum = e.nextSegNum
	e.nextSegNum++

	var buf []byte
	buf = internaljbig2.WriteSegmentHeader(buf, e.sdSegNum, 0, 0, nil, uint32(len(sdData)))
	buf = append(buf, sdData...)

	e.sdData = buf
	return buf, nil
}

// EncodePage encodes a page bitmap and returns the page stream bytes.
// The page can contain a mix of text regions (using shared symbols) and
// generic regions (raw bitmaps).
func (e *Encoder) EncodePage(page *Page) ([]byte, error) {
	if page == nil {
		return nil, fmt.Errorf("nil page")
	}

	var buf []byte

	// page information segment
	pageSegNum := e.nextSegNum
	e.nextSegNum++
	pageInfoData := internaljbig2.WritePageInfo(nil, page.Width, page.Height)
	buf = internaljbig2.WriteSegmentHeader(buf, pageSegNum, 48, 1, nil, uint32(len(pageInfoData)))
	buf = append(buf, pageInfoData...)

	// encode each region
	for _, r := range page.regions {
		switch reg := r.(type) {
		case *genericRegion:
			template := e.GenericTemplate
			if reg.opts != nil {
				template = reg.opts.Template
			}
			segData := internaljbig2.EncodeGenericRegionSegment(reg.bm, reg.x, reg.y, template, bitmap.CombOpOR)
			segNum := e.nextSegNum
			e.nextSegNum++
			buf = internaljbig2.WriteSegmentHeader(buf, segNum, 39, 1, nil, uint32(len(segData)))
			buf = append(buf, segData...)

		case *TextRegion:
			if len(e.symbols) == 0 {
				return nil, fmt.Errorf("text region with no symbols")
			}
			sdRef := e.sdSegNum

			insts := make([]internaljbig2.SymbolInstance, len(reg.Instances))
			for i, inst := range reg.Instances {
				// map original symbol ID to reordered dictionary index
				symID := inst.SymbolID
				if e.idMap != nil && symID >= 0 && symID < len(e.idMap) {
					symID = e.idMap[symID]
				}
				w, h := 0, 0
				if symID >= 0 && symID < len(e.symbols) {
					w = e.symbols[inst.SymbolID].Width()
					h = e.symbols[inst.SymbolID].Height()
				}
				insts[i] = internaljbig2.SymbolInstance{
					SymID: symID,
					T:     inst.Y,
					S:     inst.X,
					Wi:    w,
					Hi:    h,
				}
			}

			segData := internaljbig2.EncodeTextRegionSegment(
				reg.Width, reg.Height, reg.X, reg.Y, insts, len(e.symbols),
				0, false, reg.CombOp) // cornerBottomLeft, not transposed
			segNum := e.nextSegNum
			e.nextSegNum++
			buf = internaljbig2.WriteSegmentHeader(buf, segNum, 7, 1, []uint32{sdRef}, uint32(len(segData)))
			buf = append(buf, segData...)
		}
	}

	// end of page segment
	eopSegNum := e.nextSegNum
	e.nextSegNum++
	buf = internaljbig2.WriteSegmentHeader(buf, eopSegNum, 49, 1, nil, 0)

	return buf, nil
}

// Page collects regions for a single JBIG2 page (one PDF image XObject).
type Page struct {
	Width   int
	Height  int
	regions []region
}

type region interface {
	regionType() string
}

// NewPage creates a new page with the given dimensions.
func NewPage(width, height int) *Page {
	return &Page{Width: width, Height: height}
}

// AddGenericRegion adds a generic (bitmap) region to the page.
func (p *Page) AddGenericRegion(bm *bitmap.Bitmap, x, y int, opts *GenericOptions) {
	p.regions = append(p.regions, &genericRegion{bm: bm, x: x, y: y, opts: opts})
}

// AddTextRegion adds a text region (symbol instances) to the page.
func (p *Page) AddTextRegion(r *TextRegion) {
	p.regions = append(p.regions, r)
}

// GenericOptions configures generic region encoding.
type GenericOptions struct {
	Template int // 0-3
}

// TextRegion describes a text region with symbol placements.
type TextRegion struct {
	Width, Height int
	X, Y          int
	CombOp        bitmap.CombOp
	Instances     []Instance
}

func (r *TextRegion) regionType() string { return "text" }

// Instance specifies a symbol placement within a text region.
type Instance struct {
	SymbolID int
	X, Y     int
}

type genericRegion struct {
	bm   *bitmap.Bitmap
	x, y int
	opts *GenericOptions
}

func (r *genericRegion) regionType() string { return "generic" }
