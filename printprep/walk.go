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

package printprep

import (
	"bytes"
	"io"
	"maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/page"
)

// rewriteContentBytes transforms a content stream: it strips marked-content
// operators, drops the content of optional-content groups that are off, and
// re-encodes text shown in fonts that were converted to Identity-H.  It
// returns the rewritten content as a byte slice.  fc records which fonts are
// used and supplies their re-encoding maps.
//
// The second return value is the residual graphics-state nesting depth of the
// rewritten content: the number of "q" operators left unclosed at its end.  A
// caller that appends further marks can emit that many "Q" operators to return
// to the initial graphics state.  Underflowing "Q" operators (more pops than
// pushes) are dropped, so the value is never negative and the emitted content
// never pops below its own starting level.
func (c *converter) rewriteContentBytes(contents pdf.Object, resources pdf.Dict, fc *fontContext) ([]byte, int, error) {
	var props pdf.Dict
	if resources != nil {
		props, _ = pdf.CursorAt(c.x, nil).Dict(resources["Properties"])
	}

	open := func() (io.ReadCloser, error) { return c.openContent(contents) }
	it := content.NewScanner(open).NewIter()

	var buf bytes.Buffer
	var mcStack []bool // one entry per open marked-content region; true = hidden
	hiddenCount := 0
	qDepth := 0 // emitted "q" operators not yet balanced by a "Q"
	var curFont *convFont

	for name, args := range it.All() {
		switch name {
		case content.OpBeginMarkedContent, content.OpBeginMarkedContentWithProperties:
			hidden := c.markedContentHidden(name, args, props)
			mcStack = append(mcStack, hidden)
			if hidden {
				hiddenCount++
			}
			continue // strip the marked-content operator
		case content.OpEndMarkedContent:
			if n := len(mcStack); n > 0 {
				if mcStack[n-1] {
					hiddenCount--
				}
				mcStack = mcStack[:n-1]
			}
			continue // strip
		case content.OpMarkedContentPoint, content.OpMarkedContentPointWithProperties:
			continue // strip
		}

		if hiddenCount > 0 {
			continue // inside an off optional-content group
		}
		if name == content.OpXObject && c.xObjectHidden(args, resources) {
			continue
		}

		switch name {
		case content.OpTextSetFont:
			if fc != nil && len(args) >= 1 {
				if nm, ok := args[0].(pdf.Name); ok {
					curFont = fc.use(nm)
				}
			}
		case content.OpTextShow, content.OpTextShowMoveNextLine,
			content.OpTextShowMoveNextLineSetSpacing:
			// the shown string is the last operand
			if curFont != nil && curFont.codeToGID != nil && len(args) >= 1 {
				if s, ok := args[len(args)-1].(pdf.String); ok {
					args = append([]pdf.Object(nil), args...)
					args[len(args)-1] = curFont.reencode(s)
				}
			}
		case content.OpTextShowArray:
			if curFont != nil && curFont.codeToGID != nil && len(args) >= 1 {
				if arr, ok := args[0].(pdf.Array); ok {
					newArr := make(pdf.Array, len(arr))
					for i, el := range arr {
						if s, ok := el.(pdf.String); ok {
							newArr[i] = curFont.reencode(s)
						} else {
							newArr[i] = el
						}
					}
					args = []pdf.Object{newArr}
				}
			}
		}

		switch name {
		case content.OpPushGraphicsState:
			qDepth++
		case content.OpPopGraphicsState:
			if qDepth == 0 {
				continue // drop an underflowing "Q"
			}
			qDepth--
		}

		op := content.Operator{Name: name, Args: args}
		if err := op.Format(&buf); err != nil {
			return nil, 0, err
		}
		buf.WriteByte('\n')
	}
	if err := it.Err(); err != nil {
		return nil, 0, err
	}

	return buf.Bytes(), qDepth, nil
}

// closeGraphicsState appends depth "Q" operators to body, balancing that many
// "q" operators the rewritten content left open so that the content ends in
// its initial graphics state.
func closeGraphicsState(body []byte, depth int) []byte {
	for range depth {
		body = append(body, 'Q', '\n')
	}
	return body
}

// markedContentHidden reports whether a BDC/BMC marked-content region is an
// optional-content region whose group is currently off.
func (c *converter) markedContentHidden(name content.OpName, args []pdf.Object, props pdf.Dict) bool {
	if c.ocState == nil {
		return false
	}
	if name != content.OpBeginMarkedContentWithProperties || len(args) < 2 {
		return false
	}
	if tag, _ := args[0].(pdf.Name); tag != "OC" {
		return false
	}
	ocObj := args[1]
	if nm, ok := ocObj.(pdf.Name); ok {
		if props == nil {
			return false
		}
		ocObj = props[nm]
	}
	return c.ocOff(ocObj)
}

// xObjectHidden reports whether the XObject invoked by a Do operator carries an
// optional-content entry whose group is currently off.
func (c *converter) xObjectHidden(args []pdf.Object, resources pdf.Object) bool {
	if c.ocState == nil || resources == nil || len(args) < 1 {
		return false
	}
	nm, ok := args[0].(pdf.Name)
	if !ok {
		return false
	}
	cur := pdf.CursorAt(c.x, nil)
	res, _ := cur.Dict(resources)
	if res == nil {
		return false
	}
	xobjs, _ := cur.Dict(res["XObject"])
	if xobjs == nil {
		return false
	}
	stm, _ := cur.Stream(xobjs[nm])
	if stm == nil {
		return false
	}
	return c.ocOff(stm.Dict["OC"])
}

// ocOff reports whether an optional-content object (OCG or OCMD) is currently
// switched off.  A missing or unreadable entry is treated as visible.
func (c *converter) ocOff(ocObj pdf.Object) bool {
	if ocObj == nil || c.ocState == nil {
		return false
	}
	cond, err := pdf.Decode(pdf.CursorAt(c.x, nil), ocObj, oc.ExtractConditional)
	if err != nil || cond == nil {
		return false
	}
	return !cond.IsVisible(c.ocState)
}

// openContent returns a reader over the decoded, concatenated content named by
// contents, which may be a single stream or an array of streams.
func (c *converter) openContent(contents pdf.Object) (io.ReadCloser, error) {
	cur := pdf.CursorAt(c.x, nil)
	resolved, err := cur.Resolve(contents)
	if err != nil {
		return nil, err
	}
	segments, err := page.ExtractContents(cur, resolved)
	if err != nil {
		return nil, err
	}
	return page.SegmentsReader(segments), nil
}

// writeContentStream writes data as a new compressed stream object with the
// given extra dictionary entries and returns its reference.
func (c *converter) writeContentStream(data []byte, extra pdf.Dict) (pdf.Reference, error) {
	dict := pdf.Dict{}
	maps.Copy(dict, extra)
	ref := c.out.Alloc()
	stm, err := c.out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return 0, err
	}
	if _, err := stm.Write(data); err != nil {
		return 0, err
	}
	if err := stm.Close(); err != nil {
		return 0, err
	}
	return ref, nil
}
