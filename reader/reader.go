// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package reader

import (
	"math"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/textextract"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/property"
)

// A Reader reads a PDF content stream.
type Reader struct {
	x *pdf.Extractor

	// State tracks the graphics state during content stream processing.
	// Access GState for parameters (e.g., r.State.GState.CTM).
	State *content.State

	// User callbacks.
	// TODO(voss): clean up this list

	// Character is called for each character code decoded from a text-showing
	// operator.  The [font.Code] argument describes the decoded character:
	// the primary CID, the notdef fallback CID from the font's CMap (zero
	// for simple fonts and for composite fonts without a matching notdef
	// mapping), the textual representation, and the glyph widths.
	//
	// The text matrix is at the start position of the character when this
	// callback fires; the matrix advance happens after.  Use
	// [Reader.GetTextPositionDevice] for the start position.
	Character func(c font.Code) error

	TextEvent func(event TextEvent, arg float64)

	Text func(text string) error

	// UnknownOp is called for operators not handled by typed callbacks.
	// The args slice may be transient (shared with scanner buffers);
	// callers that need to retain args must clone them.
	UnknownOp func(op string, args []pdf.Object) error

	// EveryOp is called for every operator after all other processing.
	// The args slice may be transient (shared with scanner buffers);
	// callers that need to retain args must clone them.
	EveryOp func(op string, args []pdf.Object) error

	GraphicsStateSaved    func() error
	GraphicsStateRestored func() error
	XObject               func(obj graphics.XObject, ctm matrix.Matrix) error
	InlineImage           func(op content.Operator, ctm matrix.Matrix) error

	MarkedContent      func(event MarkedContentEvent, mc *graphics.MarkedContent) error
	MarkedContentStack []*graphics.MarkedContent

	// ActualText is called at the start and end of each ActualText region
	// (a marked-content span whose property list carries an ActualText
	// entry).  Nested ActualText regions are flattened: only the outermost
	// Begin/End pair is reported.  Inside such a region [Reader.InActualText]
	// returns true; consumers typically suppress per-glyph text in their
	// [Reader.Character] callback because the replacement text already
	// conveys the textual content.
	ActualText func(event ActualTextEvent, text string) error

	// actualTextStartDepth is the marked-content stack depth at which the
	// current ActualText region began, or -1 outside any region.
	actualTextStartDepth int
	actualTextValue      string

	spaceWidthCache map[font.Instance]float64

	// Position in device coordinates where the next character would naturally
	// continue after the most recent text-showing operator.  Reset at the
	// start of each page.
	prevEndX, prevEndY float64
	prevEndValid       bool
}

// TextEvent describes a transition between rendered characters that the
// reader has classified as a separator.  TextEvent values are passed to the
// [Reader.TextEvent] callback before each character that follows a
// meaningful gap or line break in the content stream.  The classification
// is based on the device-space gap between where the previous character
// ended and where the next character begins, so the reader skips spurious
// events that arise from PDF generators that wrap every glyph cluster in
// its own BT/Tm/Tj/ET sequence.
//
// In horizontal writing mode "along" means the x axis and "across" means
// the y axis; in vertical writing mode the roles are swapped.
type TextEvent uint8

const (
	// TextEventSpace indicates a gap along the writing direction, large
	// enough to be interpreted as a word separator.  The arg is the gap
	// in device units.
	TextEventSpace TextEvent = iota + 1
	// TextEventNL indicates that the text-rendering position has moved
	// across the writing direction (a new line in horizontal mode, or a
	// new column in vertical mode).
	TextEventNL
)

type MarkedContentEvent uint8

const (
	MarkedContentPoint MarkedContentEvent = iota
	MarkedContentBegin
	MarkedContentEnd
)

// ActualTextEvent indicates the boundary of an ActualText region.
type ActualTextEvent uint8

const (
	// ActualTextBegin fires when the reader enters a marked-content span
	// whose property list carries an ActualText entry.  The text argument
	// is the replacement text.
	ActualTextBegin ActualTextEvent = iota + 1
	// ActualTextEnd fires when the reader leaves the marked-content span
	// that began the current ActualText region.  The text argument is the
	// same replacement text reported at Begin.
	ActualTextEnd
)

// New creates a new Reader.  The returned Reader carries a default
// page-context graphics state with an empty resource dictionary;
// callers typically replace [Reader.State] before each page, either
// directly or via [Reader.ProcessPage].
func New(x *pdf.Extractor) *Reader {
	return &Reader{
		x:                    x,
		State:                content.NewState(content.Page, &content.Resources{}),
		MarkedContentStack:   make([]*graphics.MarkedContent, 0, 8),
		actualTextStartDepth: -1,
	}
}

// Reset resets the reader to its initial state.
// This should be used before parsing a new page.
func (r *Reader) Reset() {
	r.State = content.NewState(content.Page, &content.Resources{})
	r.MarkedContentStack = r.MarkedContentStack[:0]
	r.actualTextStartDepth = -1
	r.actualTextValue = ""
	r.prevEndValid = false
}

// InActualText reports whether the reader is currently inside an
// ActualText region whose replacement text has already been reported via
// the [Reader.ActualText] callback.  Consumers can use this from a
// [Reader.Character] callback to decide whether to emit per-glyph text.
func (r *Reader) InActualText() bool {
	return r.actualTextStartDepth != -1
}

// ProcessPage processes a decoded page's content stream.  It installs a
// fresh page-context graphics state populated with the page's resources
// and then delegates to [Reader.ProcessIter] over the page's combined
// operator iterator.
func (r *Reader) ProcessPage(pg *page.Page) error {
	r.State = content.NewState(content.Page, pg.Resources)
	return r.ProcessIter(pg.NewIter())
}

// ProcessIter processes a single-use content-stream iterator, calling the
// appropriate callback functions for each operator.  Per-stream event
// state (marked-content stack, text-position memory) is reset before
// processing begins.  After iteration, ProcessIter emits closing operators
// for any open contexts (unbalanced q/Q, BT/ET, BMC/EMC, or BX/EX).
func (r *Reader) ProcessIter(it content.Iter) error {
	r.MarkedContentStack = r.MarkedContentStack[:0]
	r.actualTextStartDepth = -1
	r.actualTextValue = ""
	r.prevEndValid = false

	for name, args := range it.All() {
		if err := r.processOperator(name, args); err != nil {
			return err
		}
	}
	if err := it.Err(); err != nil {
		return err
	}
	for _, name := range r.State.ClosingOperators() {
		if err := r.processOperator(name, nil); err != nil {
			return err
		}
	}
	return nil
}

// processOperator handles a single content stream operator.
//
// The Reader is permissive: it calls [content.State.ApplyStateChanges]
// directly (bypassing [content.State.CheckOperatorAllowed] and the
// required-state check) so that every operator advances state and every
// callback fires, matching how real-world viewers tolerate malformed
// content streams.
func (r *Reader) processOperator(name content.OpName, args []pdf.Object) error {
	// permissive reader: advance state best-effort; ignore any error
	_ = r.State.ApplyStateChanges(name, args)

	// get current graphics state (may have changed after ApplyOperator)
	p := r.State.GState

	// handle reader-specific callbacks
	switch name {

	// Text-positioning operators have no reader-specific callback: the
	// text matrix update is handled by State.ApplyOperator above, and
	// any TextEvent classification is deferred to processText, which
	// compares the start position of the next character to the natural
	// continuation of the previous text-show.
	case content.OpTextMoveOffset, content.OpTextMoveOffsetSetLeading,
		content.OpTextNextLine, content.OpTextSetMatrix:
		// nothing to do here

	case content.OpTextShow: // Tj
		if s, ok := getString(args, 0); ok && p.TextFont != nil {
			if err := r.processText(s); err != nil {
				return err
			}
		}

	case content.OpTextShowMoveNextLine: // '
		if s, ok := getString(args, 0); ok && p.TextFont != nil {
			if err := r.processText(s); err != nil {
				return err
			}
		}

	case content.OpTextShowMoveNextLineSetSpacing: // "
		if s, ok := getString(args, 2); ok && p.TextFont != nil {
			if err := r.processText(s); err != nil {
				return err
			}
		}

	case content.OpTextShowArray: // TJ
		if a, ok := getArray(args, 0); ok && p.TextFont != nil {
			for _, ai := range a {
				var d float64
				switch ai := ai.(type) {
				case pdf.String:
					if err := r.processText(ai); err != nil {
						return err
					}
				case pdf.Integer:
					d = float64(ai)
				case pdf.Real:
					d = float64(ai)
				case pdf.Number:
					d = float64(ai)
				}
				if d != 0 {
					d = d / 1000 * p.TextFontSize
					switch p.TextFont.WritingMode() {
					case font.Horizontal:
						p.TextMatrix = matrix.Translate(-d*p.TextHorizontalScaling, 0).Mul(p.TextMatrix)
					case font.Vertical:
						p.TextMatrix = matrix.Translate(0, -d).Mul(p.TextMatrix)
					}
				}
			}
		}

	// marked-content operators
	case content.OpMarkedContentPoint: // MP
		if tag, ok := getName(args, 0); ok && r.MarkedContent != nil {
			mc := &graphics.MarkedContent{
				Tag:        tag,
				Properties: nil,
			}
			if err := r.MarkedContent(MarkedContentPoint, mc); err != nil {
				return err
			}
		}

	case content.OpBeginMarkedContent: // BMC
		if tag, ok := getName(args, 0); ok && len(r.MarkedContentStack) < maxMarkedContentDepth {
			mc := &graphics.MarkedContent{
				Tag:        tag,
				Properties: nil,
			}
			r.MarkedContentStack = append(r.MarkedContentStack, mc)
			if r.MarkedContent != nil {
				if err := r.MarkedContent(MarkedContentBegin, mc); err != nil {
					return err
				}
			}
		}

	case content.OpMarkedContentPointWithProperties: // DP
		if len(args) != 2 {
			break
		}
		tag, ok1 := args[0].(pdf.Name)
		if !ok1 {
			break
		}

		mc, err := r.extractMarkedContent(tag, args[1])
		if pdf.IsMalformed(err) {
			break
		} else if err != nil {
			return err
		}

		if r.MarkedContent != nil {
			if err := r.MarkedContent(MarkedContentPoint, mc); err != nil {
				return err
			}
		}

	case content.OpBeginMarkedContentWithProperties: // BDC
		if len(args) != 2 {
			break
		}

		tag, ok1 := args[0].(pdf.Name)
		if !ok1 || len(r.MarkedContentStack) >= maxMarkedContentDepth {
			break
		}

		mc, err := r.extractMarkedContent(tag, args[1])
		if pdf.IsMalformed(err) {
			break
		} else if err != nil {
			return err
		}

		r.MarkedContentStack = append(r.MarkedContentStack, mc)
		if r.MarkedContent != nil {
			if err := r.MarkedContent(MarkedContentBegin, mc); err != nil {
				return err
			}
		}
		if r.actualTextStartDepth == -1 && mc.Properties != nil {
			if at, err := property.ListGet(mc.Properties, property.ExtractActualText); err == nil {
				r.actualTextStartDepth = len(r.MarkedContentStack)
				r.actualTextValue = at.Text
				if r.ActualText != nil {
					if err := r.ActualText(ActualTextBegin, at.Text); err != nil {
						return err
					}
				}
			}
		}

	case content.OpEndMarkedContent: // EMC
		if len(r.MarkedContentStack) > 0 {
			mc := r.MarkedContentStack[len(r.MarkedContentStack)-1]
			r.MarkedContentStack = r.MarkedContentStack[:len(r.MarkedContentStack)-1]
			if r.MarkedContent != nil {
				if err := r.MarkedContent(MarkedContentEnd, mc); err != nil {
					return err
				}
			}
			if r.actualTextStartDepth != -1 && len(r.MarkedContentStack) < r.actualTextStartDepth {
				text := r.actualTextValue
				r.actualTextStartDepth = -1
				r.actualTextValue = ""
				if r.ActualText != nil {
					if err := r.ActualText(ActualTextEnd, text); err != nil {
						return err
					}
				}
			}
		}

	// handled by typed callbacks below
	case content.OpPushGraphicsState, content.OpPopGraphicsState,
		content.OpXObject, content.OpInlineImage:

	default:
		if r.UnknownOp != nil {
			if err := r.UnknownOp(string(name), args); err != nil {
				return err
			}
		}
	}

	// typed callbacks
	switch name {
	case content.OpPushGraphicsState:
		if r.GraphicsStateSaved != nil {
			if err := r.GraphicsStateSaved(); err != nil {
				return err
			}
		}
	case content.OpPopGraphicsState:
		if r.GraphicsStateRestored != nil {
			if err := r.GraphicsStateRestored(); err != nil {
				return err
			}
		}
	case content.OpXObject:
		if r.XObject != nil && len(args) >= 1 {
			if xname, ok := args[0].(pdf.Name); ok {
				res := r.State.Resources
				if res != nil && res.XObject != nil {
					if obj := res.XObject[xname]; obj != nil {
						if err := r.XObject(obj, p.CTM); err != nil {
							return err
						}
					}
				}
			}
		}
	case content.OpInlineImage:
		if r.InlineImage != nil {
			op := content.Operator{Name: name, Args: args}
			if err := r.InlineImage(op, p.CTM); err != nil {
				return err
			}
		}
	}

	if r.EveryOp != nil {
		if err := r.EveryOp(string(name), args); err != nil {
			return err
		}
	}

	return nil
}

// extractMarkedContent extracts marked content properties from operator arguments.
// It returns the MarkedContent struct or an error if extraction fails.
func (r *Reader) extractMarkedContent(tag pdf.Name, propArg pdf.Object) (*graphics.MarkedContent, error) {
	var list property.List
	var inline bool

	if name, ok := propArg.(pdf.Name); ok {
		// Property list referenced by name from resources
		res := r.State.Resources
		if res != nil && res.Properties != nil {
			list = res.Properties[name]
		}
		inline = false
	} else {
		// Inline property dictionary
		var err error
		list, err = property.ExtractList(r.x, nil, propArg, true)
		if err != nil {
			return nil, err
		}
		inline = true
	}

	return &graphics.MarkedContent{
		Tag:        tag,
		Properties: list,
		Inline:     inline,
	}, nil
}

func (r *Reader) processText(s pdf.String) error {
	// TODO(voss): can this be merged with the corresponding code in op-text.go?
	//
	// TODO(voss): in vertical writing mode, the per-glyph (vx, vy) origin
	// offset from W2/DW2 should be applied when reporting the glyph
	// position to consumers — currently the offsets in dict.VMetrics are
	// extracted but never used.
	p := r.State.GState

	wmode := p.TextFont.WritingMode()
	visible := p.TextRenderingMode != graphics.TextRenderingModeInvisible

	// trm caches the text rendering matrix for the current TextMatrix.
	// The end-of-iteration recompute is reused as the start-of-next-
	// iteration position, so each glyph costs one matrix recompute
	// instead of two.
	var trm matrix.Matrix
	trmValid := false

	for info := range p.TextFont.Codes(s) {
		// The displacement applied to the text matrix after painting the
		// glyph; in vertical writing mode this is signed (typically
		// negative).
		var advance float64
		switch wmode {
		case font.Horizontal:
			advance = info.Width*p.TextFontSize + p.TextCharacterSpacing
			if info.UseWordSpacing {
				advance += p.TextWordSpacing
			}
			advance *= p.TextHorizontalScaling
		case font.Vertical:
			vAdv := info.VerticalAdvance
			if vAdv == 0 {
				// spec default DW2 is [880 -1000]
				vAdv = -1
			}
			advance = vAdv*p.TextFontSize + p.TextCharacterSpacing
			if info.UseWordSpacing {
				advance += p.TextWordSpacing
			}
		}

		// Classify the gap between the previous text-show end position
		// and the start of this character.  The writing direction
		// determines which axis represents "along" (potential
		// TextEventSpace) and which represents "across" (TextEventNL).
		if r.TextEvent != nil && r.prevEndValid && visible {
			if !trmValid {
				trm = p.TextRenderingMatrix()
				trmValid = true
			}
			startX, startY := trm[4], trm[5]
			effSize := math.Hypot(trm[2], trm[3])
			gapThresh := 0.5 * effSize
			alongThresh := 0.3 * r.spaceWidthDevice(p, effSize)
			// A backward jump along the writing direction by a full em
			// or more is far beyond any plausible kerning adjustment
			// (typical pair kerns peak around 0.3 em) and indicates a
			// move to a new region — a separate column, header, or
			// out-of-order glyph cluster on the same baseline.
			backThresh := effSize
			dx := startX - r.prevEndX
			dy := startY - r.prevEndY
			var along, across float64
			switch wmode {
			case font.Horizontal:
				along, across = dx, dy
			case font.Vertical:
				// Vertical text advances toward -y; a positive
				// along value means an extra gap (space) in the
				// writing direction.
				along, across = -dy, dx
			}
			switch {
			case math.Abs(across) >= gapThresh:
				r.TextEvent(TextEventNL, 0)
			case along <= -backThresh:
				r.TextEvent(TextEventNL, 0)
			case along >= alongThresh:
				r.TextEvent(TextEventSpace, along)
			}
		}

		if r.Character != nil && visible {
			err := r.Character(info)
			if err != nil {
				return err
			}
		}
		if r.Text != nil && visible {
			err := r.Text(info.Text)
			if err != nil {
				return err
			}
		}

		// Apply the advance; the cached trm is now stale.
		switch wmode {
		case font.Horizontal:
			p.TextMatrix = matrix.Translate(advance, 0).Mul(p.TextMatrix)
		case font.Vertical:
			p.TextMatrix = matrix.Translate(0, advance).Mul(p.TextMatrix)
		}
		trmValid = false

		if visible {
			trm = p.TextRenderingMatrix()
			trmValid = true
			r.prevEndX, r.prevEndY = trm[4], trm[5]
			r.prevEndValid = true
		}
	}
	return nil
}

// spaceWidthDevice returns the width of a space glyph in the current font,
// in device units.  When the font does not advertise a space, the returned
// value is a fraction of effSize.
func (r *Reader) spaceWidthDevice(p *graphics.State, effSize float64) float64 {
	sw := r.getSpaceWidth(p.TextFont) // text-space units, 1000 per em
	if sw <= 0 {
		return 0.25 * effSize
	}
	return sw / 1000 * effSize
}

// getName extracts a name from the argument slice at the given index.
func getName(args []pdf.Object, idx int) (pdf.Name, bool) {
	if idx >= len(args) {
		return "", false
	}
	if name, ok := args[idx].(pdf.Name); ok {
		return name, true
	}
	return "", false
}

// getString extracts a string from the argument slice at the given index.
func getString(args []pdf.Object, idx int) (pdf.String, bool) {
	if idx >= len(args) {
		return nil, false
	}
	if s, ok := args[idx].(pdf.String); ok {
		return s, true
	}
	return nil, false
}

// getArray extracts an array from the argument slice at the given index.
func getArray(args []pdf.Object, idx int) (pdf.Array, bool) {
	if idx >= len(args) {
		return nil, false
	}
	if a, ok := args[idx].(pdf.Array); ok {
		return a, true
	}
	return nil, false
}

// GetTextPositionDevice returns the current text position in device coordinates.
func (r *Reader) GetTextPositionDevice() (float64, float64) {
	return r.State.GState.GetTextPositionDevice()
}

func (r *Reader) getSpaceWidth(f font.Instance) float64 {
	if sw, ok := r.spaceWidthCache[f]; ok {
		return sw
	}
	sw := textextract.SpaceWidth(f)
	if r.spaceWidthCache == nil {
		r.spaceWidthCache = make(map[font.Instance]float64)
	}
	r.spaceWidthCache[f] = sw
	return sw
}

const maxMarkedContentDepth = 64
