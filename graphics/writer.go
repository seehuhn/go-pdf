// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package graphics

import (
	"fmt"
	"io"
	"math"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/internal/float"
)

// Writer writes a PDF content stream.
type Writer struct {
	Version   pdf.Version
	Content   io.Writer
	Resources *pdf.Resources
	Err       error

	currentObject objectType

	State
	stack []State

	resName  map[catRes]pdf.Name
	nameUsed map[catName]struct{}

	nesting []pairType
}

type catRes struct {
	cat resourceCategory
	res Resource
}

type catName struct {
	cat  resourceCategory
	name pdf.Name
}

type resourceCategory pdf.Name

// The valid resource categories.
// These corresponds to the fields in the Resources dictionary.
//
// See section 7.8.3 of ISO 32000-2:2020.
const (
	catExtGState  resourceCategory = "ExtGState"
	catColorSpace resourceCategory = "ColorSpace"
	catPattern    resourceCategory = "Pattern"
	catShading    resourceCategory = "Shading"
	catXObject    resourceCategory = "XObject"
	catFont       resourceCategory = "Font"
	catProperties resourceCategory = "Properties"
)

type pairType byte

const (
	pairTypeQ   pairType = iota + 1 // q ... Q
	pairTypeBT                      // BT ... ET
	pairTypeBMC                     // BMC ... EMC and BDC ... EMC
	pairTypeBX                      // BX ... EX
)

// NewWriter allocates a new Writer object.
func NewWriter(w io.Writer, v pdf.Version) *Writer {
	return &Writer{
		Version:       v,
		Content:       w,
		Resources:     &pdf.Resources{},
		currentObject: objPage,

		State: NewState(),

		resName:  make(map[catRes]pdf.Name),
		nameUsed: make(map[catName]struct{}),
	}
}

// PushGraphicsState saves the current graphics state.
//
// This implementes the PDF graphics operator "q".
func (p *Writer) PushGraphicsState() {
	var allowedStates objectType
	if p.Version >= pdf.V2_0 {
		allowedStates = objPage | objText
	} else {
		allowedStates = objPage
	}
	if !p.valid("PushGraphicsState", allowedStates) {
		return
	}

	p.nesting = append(p.nesting, pairTypeQ)

	p.stack = append(p.stack, State{
		Parameters: p.State.Parameters.Clone(),
		Set:        p.State.Set,
	})

	_, err := fmt.Fprintln(p.Content, "q")
	if p.Err == nil {
		p.Err = err
	}
}

// PopGraphicsState restores the previous graphics state.
//
// This implementes the PDF graphics operator "Q".
func (p *Writer) PopGraphicsState() {
	var allowedStates objectType
	if p.Version >= pdf.V2_0 {
		allowedStates = objPage | objText
	} else {
		allowedStates = objPage
	}
	if !p.valid("PopGraphicsState", allowedStates) {
		return
	}

	if len(p.nesting) == 0 || p.nesting[len(p.nesting)-1] != pairTypeQ {
		p.Err = fmt.Errorf("PopGraphicsState: no matching PushGraphicsState")
		return
	}
	p.nesting = p.nesting[:len(p.nesting)-1]

	n := len(p.stack) - 1
	savedState := p.stack[n]
	p.stack = p.stack[:n]
	p.State = savedState

	_, err := fmt.Fprintln(p.Content, "Q")
	if p.Err == nil {
		p.Err = err
	}
}

// Transform applies a transformation matrix to the coordinate system.
// This function modifies the current transformation matrix by multiplying the
// given matrix from the right.
//
// This implementes the PDF graphics operator "cm".
func (p *Writer) Transform(m Matrix) {
	if !p.valid("Transform", objPage) { // special graphics state
		return
	}
	p.CTM = m.Mul(p.CTM)
	_, p.Err = fmt.Fprintln(p.Content,
		float.Format(m[0], 3), float.Format(m[1], 3),
		float.Format(m[2], 3), float.Format(m[3], 3),
		float.Format(m[4], 3), float.Format(m[5], 3), "cm")
}

// SetLineWidth sets the line width.
//
// This implementes the PDF graphics operator "w".
func (p *Writer) SetLineWidth(width float64) {
	if !p.valid("SetLineWidth", objPage|objText) {
		return
	}
	if p.isSet(StateLineWidth) && nearlyEqual(width, p.LineWidth) {
		return
	}
	p.LineWidth = width
	p.Set |= StateLineWidth
	_, p.Err = fmt.Fprintln(p.Content, p.coord(width), "w")
}

// SetLineCap sets the line cap style.
//
// This implementes the PDF graphics operator "J".
func (p *Writer) SetLineCap(cap LineCapStyle) {
	if !p.valid("SetLineCap", objPage|objText) {
		return
	}
	if LineCapStyle(cap) > 2 {
		cap = 0
	}
	if p.isSet(StateLineCap) && cap == p.LineCap {
		return
	}
	p.LineCap = cap
	p.Set |= StateLineCap
	_, p.Err = fmt.Fprintln(p.Content, int(cap), "J")
}

// SetLineJoin sets the line join style.
//
// This implementes the PDF graphics operator "j".
func (p *Writer) SetLineJoin(join LineJoinStyle) {
	if !p.valid("SetLineJoin", objPage|objText) {
		return
	}
	if LineJoinStyle(join) > 2 {
		join = 0
	}
	if p.isSet(StateLineJoin) && join == p.LineJoin {
		return
	}
	p.LineJoin = join
	p.Set |= StateLineJoin
	_, p.Err = fmt.Fprintln(p.Content, int(join), "j")
}

// SetMiterLimit sets the miter limit.
func (p *Writer) SetMiterLimit(limit float64) {
	if !p.valid("SetMiterLimit", objPage|objText) {
		return
	}
	if p.isSet(StateMiterLimit) && nearlyEqual(limit, p.MiterLimit) {
		return
	}
	p.MiterLimit = limit
	p.Set |= StateMiterLimit
	_, p.Err = fmt.Fprintln(p.Content, float.Format(limit, 3), "M")
}

// SetDashPattern sets the line dash pattern.
func (p *Writer) SetDashPattern(pattern []float64, phase float64) {
	if !p.valid("SetDashPattern", objPage|objText) {
		return
	}

	if p.isSet(StateDash) &&
		sliceNearlyEqual(pattern, p.DashPattern) &&
		nearlyEqual(phase, p.DashPhase) {
		return
	}
	p.DashPattern = pattern
	p.DashPhase = phase
	p.Set |= StateDash

	_, p.Err = fmt.Fprint(p.Content, "[")
	if p.Err != nil {
		return
	}
	sep := ""
	for _, x := range pattern {
		_, p.Err = fmt.Fprint(p.Content, sep, float.Format(x, 3))
		if p.Err != nil {
			return
		}
		sep = " "
	}
	_, p.Err = fmt.Fprint(p.Content, "] ", float.Format(phase, 3), " d\n")
}

// SetRenderingIntent sets the rendering intent.
func (p *Writer) SetRenderingIntent(intent pdf.Name) {
	if !p.valid("SetRenderingIntent", objPage|objText) {
		return
	}
	if p.isSet(StateRenderingIntent) && intent == p.RenderingIntent {
		return
	}
	p.RenderingIntent = intent
	p.Set |= StateRenderingIntent
	err := intent.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, " ri")
}

// SetFlatnessTolerance sets the flatness tolerance.
func (p *Writer) SetFlatnessTolerance(flatness float64) {
	if !p.valid("SetFlatness", objPage|objText) {
		return
	}
	if p.isSet(StateFlatnessTolerance) && nearlyEqual(flatness, p.FlatnessTolerance) {
		return
	}
	p.FlatnessTolerance = flatness
	p.Set |= StateFlatnessTolerance
	_, p.Err = fmt.Fprintln(p.Content, float.Format(flatness, 3), "i")
}

// SetStrokeColor sets the stroke color in the graphics state.
// If col is nil, the stroke color is not changed.
func (p *Writer) SetStrokeColor(col color.Color) {
	if !p.valid("SetStrokeColor", objPage|objText) {
		return
	}
	if p.isSet(StateStrokeColor) && col == p.StrokeColor {
		return
	}
	p.StrokeColor = col
	p.Set |= StateStrokeColor
	p.Err = col.SetStroke(p.Content)
}

// SetFillColor sets the fill color in the graphics state.
// If col is nil, the fill color is not changed.
func (p *Writer) SetFillColor(col color.Color) {
	if !p.valid("SetFillColor", objPage|objText) {
		return
	}
	if p.isSet(StateFillColor) && col == p.FillColor {
		return
	}
	p.FillColor = col
	p.Set |= StateFillColor
	p.Err = col.SetFill(p.Content)
}

func nearlyEqual(a, b float64) bool {
	const ε = 1e-6
	return math.Abs(a-b) < ε
}

func sliceNearlyEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if nearlyEqual(x, b[i]) {
			return false
		}
	}
	return true
}

func (p *Writer) coord(x float64) string {
	// TODO(voss): Think about this some more.  Once we track the current
	// transformation matrix, we can use this to determine the number of digits
	// to keep.
	return float.Format(x, 2)
}

// GetResourceName returns the name of a resource.
// A new name is generated, if necessary, and the resource is added to the
// resource dictionary for the category.
func (p *Writer) getResourceName(category resourceCategory, r Resource) pdf.Name {
	name, ok := p.resName[catRes{category, r}]
	if ok {
		return name
	}

	var field *pdf.Dict
	var tmpl string
	switch category {
	case catFont:
		field = &p.Resources.Font
		tmpl = "F%d"
	case catExtGState:
		field = &p.Resources.ExtGState
		tmpl = "E%d"
	case catXObject:
		field = &p.Resources.XObject
		tmpl = "X%d"
	case catColorSpace:
		field = &p.Resources.ColorSpace
		tmpl = "C%d"
	case catPattern:
		field = &p.Resources.Pattern
		tmpl = "P%d"
	case catShading:
		field = &p.Resources.Shading
		tmpl = "S%d"
	case catProperties:
		field = &p.Resources.Properties
		tmpl = "MC%d"
	default:
		panic("invalid resource category " + category)
	}

	isUsed := func(name pdf.Name) bool {
		_, isUsed := p.nameUsed[catName{category, name}]
		return isUsed
	}

	origName := r.DefaultName()
	defName := origName
	if strings.HasPrefix(string(defName), "/") {
		defName = defName[1:]
	}
	if origName != "" && !isUsed(defName) {
		name = defName
	} else {
		var numUsed int
		for item := range p.nameUsed {
			if item.cat == category {
				numUsed++
			}
		}
		for k := numUsed + 1; ; k-- {
			name = pdf.Name(fmt.Sprintf(tmpl, k))
			if !isUsed(name) {
				break
			}
		}
	}
	p.resName[catRes{category, r}] = name
	p.nameUsed[catName{category, name}] = struct{}{}

	if *field == nil {
		*field = pdf.Dict{}
	}
	(*field)[name] = r.PDFObject()

	return name
}

// See Figure 9 (p. 113) of PDF 32000-1:2008.
type objectType int

const (
	objPage objectType = 1 << iota
	objPath
	objText
	objClippingPath
	objShading
	objInlineImage
	objExternal
)

func (s objectType) String() string {
	switch s {
	case objPage:
		return "page"
	case objPath:
		return "path"
	case objText:
		return "text"
	case objClippingPath:
		return "clipping path"
	case objShading:
		return "shading"
	case objInlineImage:
		return "inline image"
	case objExternal:
		return "external"
	default:
		return fmt.Sprintf("objectType(%d)", s)
	}
}

// valid returns true, if the current graphics object is one of the given types.
// Otherwise it sets p.Err and returns false.
func (p *Writer) valid(cmd string, ss objectType) bool {
	if p.Err != nil {
		return false
	}

	if p.currentObject&ss != 0 {
		return true
	}

	p.Err = fmt.Errorf("unexpected state %q for %q", p.currentObject, cmd)
	return false
}

// Res represents a named PDF resource.
type Res struct {
	DefName pdf.Name
	Data    pdf.Reference // TODO(voss): can this be pdf.Object?
}

// DefaultName implements the [Resource] interface.
func (r Res) DefaultName() pdf.Name {
	return r.DefName
}

// PDFObject implements the [Resource] interface.
func (r Res) PDFObject() pdf.Object {
	return r.Data
}

// Resource represents the different PDF Resource types.
// Implementations of this must be "comparable".
type Resource interface {
	DefaultName() pdf.Name // return "" to choose names automatically
	PDFObject() pdf.Object // value to use in the resource dictionary
}
