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
	"errors"
	"fmt"
	"maps"
	"sort"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/bitmap"
	internaljbig2 "seehuhn.de/go/pdf/internal/filter/jbig2"
)

// Globals holds shared JBIG2 dictionaries (symbols, patterns) that may
// be referenced from one or more [Image]s.  Sharing globals across
// images avoids duplicating dictionary data in the PDF file.
//
// Globals implements [pdf.Embedder]; two [Image]s sharing the same
// *Globals pointer produce a single globals stream in the output PDF.
// Once embedded, the Globals is frozen and no further symbols or
// patterns can be added.
type Globals struct {
	// SymbolTemplate selects the arithmetic coding template used when
	// encoding shared symbol dictionaries (0-3).  Defaults to 0.
	SymbolTemplate int

	// PatternTemplate selects the arithmetic coding template used when
	// encoding shared pattern dictionaries (0-3).  Defaults to 0.
	PatternTemplate int

	symbols  []*bitmap.Bitmap
	patterns [][]*bitmap.Bitmap // one entry per pattern dictionary

	// internal segment numbers (set at embed time)
	symbolSegNum  uint32 // segment number of the symbol dictionary
	patternSegNum []uint32

	// idMap[i] = reordered index of the symbol added with AddSymbol(i)
	symIDMap []int

	frozen    bool
	encoded   []byte // cached output
	nextSegNo uint32 // next segment number to assign at embed time
}

// NewGlobals returns an empty globals container.
func NewGlobals() *Globals {
	return &Globals{}
}

// AddSymbol appends a shared symbol to the symbol dictionary and returns
// its ID.  All symbols added to a Globals form a single symbol
// dictionary segment.  Returns an error if the Globals has already been
// embedded.
func (g *Globals) AddSymbol(bm *bitmap.Bitmap) (int, error) {
	if g.frozen {
		return 0, errors.New("jbig2: Globals frozen; cannot add symbols after embedding")
	}
	id := len(g.symbols)
	g.symbols = append(g.symbols, bm)
	return id, nil
}

// AddPatternDict appends a pattern dictionary to the globals and
// returns its ID.  All patterns within a single dictionary must have
// the same dimensions.  Returns an error if the Globals has already
// been embedded.
func (g *Globals) AddPatternDict(patterns []*bitmap.Bitmap) (int, error) {
	if g.frozen {
		return 0, errors.New("jbig2: Globals frozen; cannot add patterns after embedding")
	}
	if len(patterns) == 0 {
		return 0, errors.New("jbig2: pattern dictionary must contain at least one pattern")
	}
	w, h := patterns[0].Width(), patterns[0].Height()
	for i, p := range patterns {
		if p.Width() != w || p.Height() != h {
			return 0, fmt.Errorf("jbig2: pattern %d has dimensions %dx%d, expected %dx%d",
				i, p.Width(), p.Height(), w, h)
		}
	}
	id := len(g.patterns)
	g.patterns = append(g.patterns, patterns)
	return id, nil
}

// Embed writes the globals stream to the PDF output and returns a
// reference to it.  The Globals is frozen after this call.
//
// Embed is the entry point used by the PDF writer's resource manager.
// Users do not normally call it directly; instead, [Image] calls it
// during its own Embed method.
func (g *Globals) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	data, err := g.encode()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("jbig2: Globals is empty")
	}

	ref := rm.Alloc()
	w, err := rm.Out().OpenStream(ref, pdf.Dict{})
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return ref, nil
}

// hasSegments reports whether the Globals contains any segment data.
func (g *Globals) hasSegments() bool {
	return len(g.symbols) > 0 || len(g.patterns) > 0
}

// encode materialises the Globals into a byte slice of JBIG2 segments.
// The result is cached; subsequent calls return the same bytes.  encode
// freezes the Globals.
func (g *Globals) encode() ([]byte, error) {
	if g.encoded != nil {
		return g.encoded, nil
	}
	if !g.hasSegments() {
		g.frozen = true
		return nil, nil
	}

	var buf []byte
	g.nextSegNo = 0

	// symbol dictionary
	if len(g.symbols) > 0 {
		reordered, idMap := sortSymbols(g.symbols)
		g.symIDMap = idMap

		sdData := internaljbig2.EncodeSymbolDictSegment(reordered, g.SymbolTemplate)
		g.symbolSegNum = g.nextSegNo
		g.nextSegNo++
		buf = internaljbig2.WriteSegmentHeader(buf, g.symbolSegNum, 0, 0, nil, uint32(len(sdData)))
		buf = append(buf, sdData...)
	}

	// pattern dictionaries
	for _, pats := range g.patterns {
		pdData := internaljbig2.EncodePatternDictSegment(pats, g.PatternTemplate)
		segNum := g.nextSegNo
		g.nextSegNo++
		g.patternSegNum = append(g.patternSegNum, segNum)
		buf = internaljbig2.WriteSegmentHeader(buf, segNum, 16, 0, nil, uint32(len(pdData)))
		buf = append(buf, pdData...)
	}

	g.encoded = buf
	g.frozen = true
	return buf, nil
}

// sortSymbols groups symbols by height class and returns the reordered
// slice together with an old-ID-to-new-index mapping.
func sortSymbols(symbols []*bitmap.Bitmap) ([]*bitmap.Bitmap, []int) {
	type symEntry struct {
		idx    int
		width  int
		height int
	}
	entries := make([]symEntry, len(symbols))
	for i, s := range symbols {
		entries[i] = symEntry{i, s.Width(), s.Height()}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].height != entries[j].height {
			return entries[i].height < entries[j].height
		}
		return entries[i].width < entries[j].width
	})

	reordered := make([]*bitmap.Bitmap, len(symbols))
	idMap := make([]int, len(symbols))
	for i, ent := range entries {
		reordered[i] = symbols[ent.idx]
		idMap[ent.idx] = i
	}
	return reordered, idMap
}

// Image encodes a single JBIG2 image stream as a sequence of regions.
// It implements [graphics.ImageData] and is used as the Data field
// of an [seehuhn.de/go/pdf/graphics/image.Dict] (for a 1-bit
// DeviceGray image) or the Source field of an
// [seehuhn.de/go/pdf/graphics/image.Mask] (for a stencil mask).
type Image struct {
	width, height int
	globals       *Globals
	ops           []func(ctx *encodeCtx) error
	encoded       []byte // cached output
}

// NewImage creates an empty JBIG2 image stream of the given pixel
// dimensions.  If globals is non-nil, region segments in this image may
// reference the symbol dictionary, pattern dictionaries and Huffman
// tables that globals contains.  If globals is nil, the image is
// self-contained.
func NewImage(width, height int, globals *Globals) *Image {
	return &Image{
		width:   width,
		height:  height,
		globals: globals,
	}
}

// Bounds returns the pixel dimensions of the image.
func (im *Image) Bounds() rect.IntRect {
	return rect.IntRect{XMin: 0, YMin: 0, XMax: im.width, YMax: im.height}
}

// GenericOptions configures the encoding of a generic region.
type GenericOptions struct {
	// Template selects the arithmetic coding template (0-3).
	Template int

	// TPGDOn enables typical prediction, which reduces output size for
	// images with many duplicate rows.
	TPGDOn bool

	// ExtTemplate, when true with Template=0, selects the extended
	// template with 12 adaptive pixels.
	ExtTemplate bool

	// UseMMR selects the modified-modified-READ (Group 4 facsimile)
	// coder instead of the arithmetic coder.  Template, TPGDOn and
	// ExtTemplate are ignored when UseMMR is true.
	UseMMR bool

	// CombOp controls how the region is combined with the underlying
	// page pixels.  Defaults to CombOpOR.
	CombOp bitmap.CombOp
}

// AddGenericRegion appends a generic region to the image.  The bitmap
// is placed at (x, y) relative to the page origin.  If opts is nil,
// default options (template 0, arithmetic coder, OR combination) are
// used.
func (im *Image) AddGenericRegion(bm *bitmap.Bitmap, x, y int, opts *GenericOptions) {
	var o GenericOptions
	if opts != nil {
		o = *opts
	}
	im.ops = append(im.ops, func(ctx *encodeCtx) error {
		var segData []byte
		var err error
		if o.UseMMR {
			segData, err = internaljbig2.EncodeGenericRegionSegmentMMR(bm, x, y, o.CombOp)
			if err != nil {
				return err
			}
		} else {
			segData = internaljbig2.EncodeGenericRegionSegment(bm, x, y, o.Template, o.CombOp, o.TPGDOn, o.ExtTemplate)
		}
		return ctx.writeRegionSegment(39, nil, segData) // immediate lossless generic
	})
}

// RefinementOptions configures the encoding of a generic refinement
// region.
type RefinementOptions struct {
	// Template selects the refinement coding template (0 or 1).
	Template int

	// TPGROn enables typical prediction for refinement.
	TPGROn bool

	// CombOp controls how the region is combined with the underlying
	// page pixels.  Defaults to CombOpOR.
	CombOp bitmap.CombOp
}

// AddRefinementRegion appends a generic refinement region to the image.
// The bitmap bm is encoded as a refinement of ref.  The reference
// bitmap is first written as a generic region so that the decoder can
// extract it from the page buffer when decoding the refinement.
func (im *Image) AddRefinementRegion(bm, ref *bitmap.Bitmap, x, y int, opts *RefinementOptions) {
	var o RefinementOptions
	if opts != nil {
		o = *opts
	}
	im.ops = append(im.ops, func(ctx *encodeCtx) error {
		// write the reference bitmap as a generic region first
		refSeg := internaljbig2.EncodeGenericRegionSegment(ref, x, y, 0, bitmap.CombOpReplace, false, false)
		if err := ctx.writeRegionSegment(39, nil, refSeg); err != nil {
			return err
		}

		// now encode the refinement relative to the page buffer
		segData := internaljbig2.EncodeRefinementRegionSegment(bm, ref, x, y, o.Template, o.CombOp, o.TPGROn)
		return ctx.writeRegionSegment(42, nil, segData)
	})
}

// TextRegionInstance describes a single symbol placement within a text
// region.  SymbolID is an index into the shared symbol dictionary of
// the enclosing [Image]'s [Globals].  (X, Y) is the reference point in
// page coordinates, with the corner of the symbol determined by
// [TextRegion.RefCorner].
type TextRegionInstance struct {
	SymbolID int
	X, Y     int
}

// RefCorner identifies which corner of a symbol lies at the
// instance's reference point.
type RefCorner int

// Reference corners for text region instances.
const (
	RefBottomLeft  RefCorner = 0
	RefTopLeft     RefCorner = 1
	RefBottomRight RefCorner = 2
	RefTopRight    RefCorner = 3
)

// TextRegion describes a text region segment that places instances of
// shared symbols onto the page.
type TextRegion struct {
	// Width and Height are the region's pixel dimensions.
	Width, Height int

	// X and Y are the top-left corner of the region within the page.
	X, Y int

	// Instances lists the symbol placements.  Each SymbolID indexes
	// into the enclosing [Image]'s [Globals] symbol dictionary.
	Instances []TextRegionInstance

	// RefCorner identifies which corner of each symbol lies at its
	// instance's reference point.  Defaults to [RefBottomLeft].
	RefCorner RefCorner

	// Transposed, when true, swaps the T and S axes of the text
	// region (strips run vertically instead of horizontally).
	Transposed bool

	// CombOp controls how the region is combined with the underlying
	// page pixels.  Defaults to CombOpOR.
	CombOp bitmap.CombOp

	// Strips must be 1, 2, 4, or 8.  Instances within the same strip
	// (T values differing by less than Strips) share a strip row.
	// Defaults to 1.
	Strips int

	// DSOffset is added to the inline coordinate for non-first
	// instances in a strip.
	DSOffset int

	// DefPixel selects the initial pixel value of the region
	// (0 = white, 1 = black).
	DefPixel int

	// UseHuffman selects Huffman coding instead of the arithmetic
	// coder.
	UseHuffman bool
}

// AddTextRegion appends a text region to the image.  The region
// references symbols from the enclosing [Image]'s [Globals].  A nil
// Globals or an empty symbol dictionary returns an error at embed time.
func (im *Image) AddTextRegion(r *TextRegion) {
	reg := *r
	im.ops = append(im.ops, func(ctx *encodeCtx) error {
		if im.globals == nil || len(im.globals.symbols) == 0 {
			return errors.New("jbig2: text region requires Globals with at least one symbol")
		}
		strips := reg.Strips
		if strips <= 0 {
			strips = 1
		}

		// map user-facing SymbolIDs (addition order) to the reordered
		// dictionary indices used by the encoder.
		idMap := im.globals.symIDMap
		symbols := im.globals.symbols
		insts := make([]internaljbig2.SymbolInstance, len(reg.Instances))
		for i, inst := range reg.Instances {
			origID := inst.SymbolID
			if origID < 0 || origID >= len(symbols) {
				return fmt.Errorf("jbig2: text region SymbolID %d out of range", origID)
			}
			symID := origID
			if idMap != nil {
				symID = idMap[origID]
			}
			bm := symbols[origID]
			insts[i] = internaljbig2.SymbolInstance{
				SymID: symID,
				T:     inst.Y,
				S:     inst.X,
				Wi:    bm.Width(),
				Hi:    bm.Height(),
			}
		}

		var segData []byte
		var err error
		if reg.UseHuffman {
			segData, err = internaljbig2.EncodeTextRegionSegmentHuffman(
				reg.Width, reg.Height, reg.X, reg.Y, insts, symbols,
				int(reg.RefCorner), reg.Transposed, reg.CombOp,
				strips, reg.DSOffset, reg.DefPixel)
			if err != nil {
				return err
			}
		} else {
			segData = internaljbig2.EncodeTextRegionSegment(
				reg.Width, reg.Height, reg.X, reg.Y, insts, symbols,
				int(reg.RefCorner), reg.Transposed, reg.CombOp,
				strips, reg.DSOffset, reg.DefPixel)
		}

		// text region references the shared symbol dictionary
		refs := []uint32{im.globals.symbolSegNum}
		return ctx.writeRegionSegment(7, refs, segData) // immediate lossless text
	})
}

// HalftoneRegion describes a halftone region segment that renders a
// gray-scale image using a shared pattern dictionary.
type HalftoneRegion struct {
	// Width and Height are the region's pixel dimensions.
	Width, Height int

	// PatternDictID selects which pattern dictionary from the shared
	// [Globals] to use.
	PatternDictID int

	// GrayValues is the row-major gray-scale image to halftone, with
	// dimensions GridWidth x GridHeight.
	GrayValues []int

	// GridWidth and GridHeight are the dimensions of GrayValues.
	GridWidth, GridHeight int

	// GridX, GridY is the grid origin in pixels.
	GridX, GridY int

	// GridVX, GridVY is the grid column vector in pixels.
	// The row vector is perpendicular: (-GridVY, GridVX).
	GridVX, GridVY int

	// Template selects the arithmetic coding template used for the
	// gray-scale bitplane encoding (0-3).
	Template int

	// CombOp controls how the region is combined with the underlying
	// page pixels.
	CombOp bitmap.CombOp

	// EnableSkip, when true, marks grid cells falling entirely outside
	// the region as skipped.
	EnableSkip bool

	// UseMMR selects the MMR (Group 4) coder instead of the arithmetic
	// coder for the gray-scale bitplanes.
	UseMMR bool
}

// AddHalftoneRegion appends a halftone region to the image.
func (im *Image) AddHalftoneRegion(r *HalftoneRegion) {
	reg := *r
	im.ops = append(im.ops, func(ctx *encodeCtx) error {
		if im.globals == nil || reg.PatternDictID >= len(im.globals.patterns) {
			return fmt.Errorf("jbig2: halftone region PatternDictID %d out of range", reg.PatternDictID)
		}
		pats := im.globals.patterns[reg.PatternDictID]
		hpw, hph := pats[0].Width(), pats[0].Height()
		numPats := len(pats)
		patSegNum := im.globals.patternSegNum[reg.PatternDictID]

		// the JBIG2 spec stores grid coordinates scaled by 256
		hgx := reg.GridX * 256
		hgy := reg.GridY * 256
		hrx := reg.GridVX * 256
		hry := reg.GridVY * 256

		var segData []byte
		if reg.UseMMR {
			d, err := internaljbig2.EncodeHalftoneRegionSegmentMMR(
				reg.Width, reg.Height,
				reg.GrayValues, reg.GridWidth, reg.GridHeight,
				hgx, hgy, hrx, hry,
				numPats, reg.CombOp, reg.EnableSkip)
			if err != nil {
				return err
			}
			segData = d
		} else {
			segData = internaljbig2.EncodeHalftoneRegionSegment(
				reg.Width, reg.Height,
				reg.GrayValues, reg.GridWidth, reg.GridHeight,
				hgx, hgy, hrx, hry,
				numPats, reg.Template, reg.CombOp,
				reg.EnableSkip, hpw, hph)
		}

		return ctx.writeRegionSegment(22, []uint32{patSegNum}, segData) // immediate halftone
	})
}

// encodeCtx carries the running segment-number counter and accumulated
// output buffer while encoding an [Image].
type encodeCtx struct {
	buf       []byte
	nextSegNo uint32
}

// writeRegionSegment appends a region segment (page association 1) with
// the given segment type, referred-to segment numbers, and data.
func (ctx *encodeCtx) writeRegionSegment(segType int, refs []uint32, data []byte) error {
	segNo := ctx.nextSegNo
	ctx.nextSegNo++
	ctx.buf = internaljbig2.WriteSegmentHeader(ctx.buf, segNo, segType, 1, refs, uint32(len(data)))
	ctx.buf = append(ctx.buf, data...)
	return nil
}

// encode materialises the image into a byte slice of JBIG2 segments.
// The result is cached; subsequent calls return the same bytes.  The
// leading segment is a page information segment with page association
// 1; no end-of-page or end-of-file segments are emitted, per PDF
// 2.0 section 7.4.7.
func (im *Image) encode() ([]byte, error) {
	if im.encoded != nil {
		return im.encoded, nil
	}

	ctx := &encodeCtx{}
	// Image-local segments start after any globals segments so that
	// segment numbers remain unique across the whole JBIG2 bitstream
	// (globals + page).  When there are globals, the first image
	// segment number equals globals.nextSegNo; otherwise it starts
	// from 0.  The text region references the globals' symbol segment
	// number directly.
	if im.globals != nil {
		ctx.nextSegNo = im.globals.nextSegNo
	}

	// page information segment
	pageInfoData := internaljbig2.WritePageInfo(nil, im.width, im.height)
	segNo := ctx.nextSegNo
	ctx.nextSegNo++
	ctx.buf = internaljbig2.WriteSegmentHeader(ctx.buf, segNo, 48, 1, nil, uint32(len(pageInfoData)))
	ctx.buf = append(ctx.buf, pageInfoData...)

	for _, op := range im.ops {
		if err := op(ctx); err != nil {
			return nil, err
		}
	}

	im.encoded = ctx.buf
	return ctx.buf, nil
}

// Pixels returns the raw, uncompressed 1-bit pixel data.
// Each row starts at a new byte boundary, with the high-order bit first.
func (im *Image) Pixels() ([]byte, error) {
	// encode globals first so that segment numbers (patternSegNum,
	// symbolSegNum) are assigned before the image ops reference them
	var globalsData []byte
	if im.globals != nil && im.globals.hasSegments() {
		var err error
		globalsData, err = im.globals.encode()
		if err != nil {
			return nil, err
		}
	}

	data, err := im.encode()
	if err != nil {
		return nil, err
	}

	bm, err := internaljbig2.Decode(globalsData, data)
	if err != nil {
		return nil, err
	}

	// convert bitmap to raw pixel bytes with each row at a byte boundary,
	// inverting to match the normal PDF convention (0=black)
	stride := (im.width + 7) / 8
	out := make([]byte, stride*im.height)
	for y := range im.height {
		for i := range stride {
			out[y*stride+i] = bm.Pix[y*bm.Stride+i] ^ 0xFF
		}
	}
	return out, nil
}

// WriteStream implements [graphics.ImageData].  It writes the JBIG2
// encoded data to the PDF stream and embeds any referenced globals
// stream.
func (im *Image) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	dict = maps.Clone(dict)

	// embed globals first, if any, so that globals.encode() assigns
	// segment numbers before we encode the image.
	var decodeParms pdf.Dict
	if im.globals != nil && im.globals.hasSegments() {
		globalsRef, err := rm.Embed(im.globals)
		if err != nil {
			return err
		}
		decodeParms = pdf.Dict{"JBIG2Globals": globalsRef}
	}

	data, err := im.encode()
	if err != nil {
		return err
	}

	// Pre-set Filter / DecodeParms in the dict and pass no filters to
	// OpenStream, so that the data is written raw.  This is the same
	// pattern used for pre-encoded JPEG data in
	// feature-tests/image/funny-jpeg.
	dict["Filter"] = pdf.Name("JBIG2Decode")
	if decodeParms != nil {
		dict["DecodeParms"] = decodeParms
	}

	w, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// Compile-time interface checks.
var (
	_ graphics.ImageData = (*Image)(nil)
	_ pdf.Embedder       = (*Globals)(nil)
)
