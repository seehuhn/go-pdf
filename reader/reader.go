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
	"io"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/property"
)

// A Reader reads a PDF content stream.
type Reader struct {
	R pdf.Getter
	x *pdf.Extractor

	// State tracks the graphics state during content stream processing.
	// Access GState for parameters (e.g., r.State.GState.CTM).
	State *content.State

	// User callbacks.
	// TODO(voss): clean up this list
	Character func(cid cid.CID, text string) error
	TextEvent func(event TextEvent, arg float64)

	Text      func(text string) error
	UnknownOp func(op string, args []pdf.Object) error
	EveryOp   func(op string, args []pdf.Object) error

	MarkedContent      func(event MarkedContentEvent, mc *graphics.MarkedContent) error
	MarkedContentStack []*graphics.MarkedContent
}

type TextEvent uint8

const (
	TextEventNone TextEvent = iota
	TextEventSpace
	TextEventNL
	TextEventMove
)

type MarkedContentEvent uint8

const (
	MarkedContentPoint MarkedContentEvent = iota
	MarkedContentBegin
	MarkedContentEnd
)

// New creates a new Reader.
func New(r pdf.Getter) *Reader {
	return &Reader{
		R:                  r,
		x:                  pdf.NewExtractor(r),
		MarkedContentStack: make([]*graphics.MarkedContent, 0, 8),
	}
}

// Reset resets the reader to its initial state.
// This should be used before parsing a new page.
func (r *Reader) Reset() {
	r.State = content.NewState(content.Page, &content.Resources{})
	r.MarkedContentStack = r.MarkedContentStack[:0]
}

// ParsePage parses a page, and calls the appropriate callback functions.
func (r *Reader) ParsePage(page pdf.Object, ctm matrix.Matrix) error {
	pageDict, err := pdf.GetDictTyped(r.R, page, "Page")
	if err != nil {
		return err
	}

	// TODO(voss): do we need to worry about inherited resources? There is some
	// code in seehuhn.de/go/pdf/pagetree that copies inherited resources from
	// the parent, but this needs to be checked and documented.  Also, it
	// reduces generality of the ParsePage method.
	res, err := extract.Resources(r.x, pageDict["Resources"])
	if err != nil {
		return err
	}
	if res == nil {
		res = &content.Resources{}
	}

	r.State = content.NewState(content.Page, res)
	r.State.GState.CTM = ctm
	r.MarkedContentStack = r.MarkedContentStack[:0]

	contentReader, err := pagetree.ContentStream(r.R, page)
	if err != nil {
		return err
	}
	return r.ParseContentStream(contentReader)
}

// ParseContentStream parses a PDF content stream.
func (r *Reader) ParseContentStream(in io.Reader) error {
	v := pdf.GetVersion(r.R)
	stream, err := content.ReadStream(in, v, content.Page, r.State.Resources)
	if err != nil {
		return err
	}
	return r.processStream(stream)
}

func (r *Reader) processStream(stream content.Stream) error {
	for _, op := range stream {
		origArgs := op.Args

		// Apply state changes first
		_ = r.State.ApplyOperator(op.Name, op.Args) // ignore errors in permissive reader

		// Get current graphics state (may have changed after ApplyOperator)
		p := r.State.GState

		// Handle reader-specific callbacks
		switch op.Name {

		// Text-positioning operators - emit TextEvent callbacks
		case content.OpTextMoveOffset, content.OpTextMoveOffsetSetLeading, content.OpTextNextLine: // Td, TD, T*
			if r.TextEvent != nil {
				r.TextEvent(TextEventNL, 0)
			}

		case content.OpTextSetMatrix: // Tm
			if r.TextEvent != nil {
				r.TextEvent(TextEventMove, 0)
			}

		// Text-showing operators
		case content.OpTextShow: // Tj
			if s, ok := getString(op.Args, 0); ok && p.TextFont != nil {
				err := r.processText(s)
				if err != nil {
					return err
				}
			}

		case content.OpTextShowMoveNextLine: // '
			// State already moved to next line
			if r.TextEvent != nil {
				r.TextEvent(TextEventNL, 0)
			}
			if s, ok := getString(op.Args, 0); ok && p.TextFont != nil {
				err := r.processText(s)
				if err != nil {
					return err
				}
			}

		case content.OpTextShowMoveNextLineSetSpacing: // "
			// State already set spacing and moved to next line
			if r.TextEvent != nil {
				r.TextEvent(TextEventNL, 0)
			}
			if s, ok := getString(op.Args, 2); ok && p.TextFont != nil {
				err := r.processText(s)
				if err != nil {
					return err
				}
			}

		case content.OpTextShowArray: // TJ
			if a, ok := getArray(op.Args, 0); ok && p.TextFont != nil {
				for _, ai := range a {
					var d float64
					switch ai := ai.(type) {
					case pdf.String:
						err := r.processText(ai)
						if err != nil {
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
						if d < 0 && r.TextEvent != nil {
							r.TextEvent(TextEventSpace, -d)
						}

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

		// Marked-content operators
		case content.OpMarkedContentPoint: // MP
			if tag, ok := getName(op.Args, 0); ok && r.MarkedContent != nil {
				mc := &graphics.MarkedContent{
					Tag:        tag,
					Properties: nil,
				}
				err := r.MarkedContent(MarkedContentPoint, mc)
				if err != nil {
					return err
				}
			}

		case content.OpBeginMarkedContent: // BMC
			if tag, ok := getName(op.Args, 0); ok && len(r.MarkedContentStack) < maxMarkedContentDepth {
				mc := &graphics.MarkedContent{
					Tag:        tag,
					Properties: nil,
				}
				r.MarkedContentStack = append(r.MarkedContentStack, mc)
				if r.MarkedContent != nil {
					err := r.MarkedContent(MarkedContentBegin, mc)
					if err != nil {
						return err
					}
				}
			}

		case content.OpMarkedContentPointWithProperties: // DP
			if len(op.Args) != 2 {
				break
			}
			tag, ok1 := op.Args[0].(pdf.Name)
			if !ok1 {
				break
			}

			mc, err := r.extractMarkedContent(tag, op.Args[1])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return err
			}

			if r.MarkedContent != nil {
				err := r.MarkedContent(MarkedContentPoint, mc)
				if err != nil {
					return err
				}
			}

		case content.OpBeginMarkedContentWithProperties: // BDC
			if len(op.Args) != 2 {
				break
			}

			tag, ok1 := op.Args[0].(pdf.Name)
			if !ok1 || len(r.MarkedContentStack) >= maxMarkedContentDepth {
				break
			}

			mc, err := r.extractMarkedContent(tag, op.Args[1])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return err
			}

			r.MarkedContentStack = append(r.MarkedContentStack, mc)
			if r.MarkedContent != nil {
				err := r.MarkedContent(MarkedContentBegin, mc)
				if err != nil {
					return err
				}
			}

		case content.OpEndMarkedContent: // EMC
			if len(r.MarkedContentStack) > 0 {
				mc := r.MarkedContentStack[len(r.MarkedContentStack)-1]
				r.MarkedContentStack = r.MarkedContentStack[:len(r.MarkedContentStack)-1]
				if r.MarkedContent != nil {
					err := r.MarkedContent(MarkedContentEnd, mc)
					if err != nil {
						return err
					}
				}
			}

		default:
			if r.UnknownOp != nil {
				err := r.UnknownOp(string(op.Name), op.Args)
				if err != nil {
					return err
				}
			}
		}

		if r.EveryOp != nil {
			err := r.EveryOp(string(op.Name), origArgs)
			if err != nil {
				return err
			}
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
		list, err = property.ExtractList(r.x, propArg)
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
	p := r.State.GState

	wmode := p.TextFont.WritingMode()
	for info := range p.TextFont.Codes(s) {
		width := info.Width*p.TextFontSize + p.TextCharacterSpacing
		if info.UseWordSpacing {
			width += p.TextWordSpacing
		}
		if wmode == font.Horizontal {
			width *= p.TextHorizontalScaling
		}

		if r.Character != nil && p.TextRenderingMode != graphics.TextRenderingModeInvisible {
			err := r.Character(info.CID, info.Text)
			if err != nil {
				return err
			}
		}
		if r.Text != nil && p.TextRenderingMode != graphics.TextRenderingModeInvisible {
			err := r.Text(info.Text)
			if err != nil {
				return err
			}
		}

		switch wmode {
		case font.Horizontal:
			p.TextMatrix = matrix.Translate(width, 0).Mul(p.TextMatrix)
		case font.Vertical:
			p.TextMatrix = matrix.Translate(0, width).Mul(p.TextMatrix)
		}
	}
	return nil
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

const maxMarkedContentDepth = 64
