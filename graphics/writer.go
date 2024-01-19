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
func (w *Writer) PushGraphicsState() {
	var allowedStates objectType
	if w.Version >= pdf.V2_0 {
		allowedStates = objPage | objText
	} else {
		allowedStates = objPage
	}
	if !w.isValid("PushGraphicsState", allowedStates) {
		return
	}

	w.nesting = append(w.nesting, pairTypeQ)

	w.stack = append(w.stack, State{
		Parameters: w.State.Parameters.Clone(),
		Set:        w.State.Set,
	})

	_, err := fmt.Fprintln(w.Content, "q")
	if w.Err == nil {
		w.Err = err
	}
}

// PopGraphicsState restores the previous graphics state.
//
// This implementes the PDF graphics operator "Q".
func (w *Writer) PopGraphicsState() {
	var allowedStates objectType
	if w.Version >= pdf.V2_0 {
		allowedStates = objPage | objText
	} else {
		allowedStates = objPage
	}
	if !w.isValid("PopGraphicsState", allowedStates) {
		return
	}

	if len(w.nesting) == 0 || w.nesting[len(w.nesting)-1] != pairTypeQ {
		w.Err = fmt.Errorf("PopGraphicsState: no matching PushGraphicsState")
		return
	}
	w.nesting = w.nesting[:len(w.nesting)-1]

	n := len(w.stack) - 1
	savedState := w.stack[n]
	w.stack = w.stack[:n]
	w.State = savedState

	_, err := fmt.Fprintln(w.Content, "Q")
	if w.Err == nil {
		w.Err = err
	}
}

// Transform applies a transformation matrix to the coordinate system.
// This function modifies the current transformation matrix by multiplying the
// given matrix from the right.
//
// This implementes the PDF graphics operator "cm".
func (w *Writer) Transform(m Matrix) {
	if !w.isValid("Transform", objPage) { // special graphics state
		return
	}
	w.CTM = m.Mul(w.CTM)
	_, w.Err = fmt.Fprintln(w.Content,
		float.Format(m[0], 3), float.Format(m[1], 3),
		float.Format(m[2], 3), float.Format(m[3], 3),
		float.Format(m[4], 3), float.Format(m[5], 3), "cm")
}

// SetLineWidth sets the line width.
//
// This implementes the PDF graphics operator "w".
func (w *Writer) SetLineWidth(width float64) {
	if !w.isValid("SetLineWidth", objPage|objText) {
		return
	}
	if w.isSet(StateLineWidth) && nearlyEqual(width, w.LineWidth) {
		return
	}
	w.LineWidth = width
	w.Set |= StateLineWidth
	_, w.Err = fmt.Fprintln(w.Content, w.coord(width), "w")
}

// SetLineCap sets the line cap style.
//
// This implementes the PDF graphics operator "J".
func (w *Writer) SetLineCap(cap LineCapStyle) {
	if !w.isValid("SetLineCap", objPage|objText) {
		return
	}
	if LineCapStyle(cap) > 2 {
		cap = 0
	}
	if w.isSet(StateLineCap) && cap == w.LineCap {
		return
	}
	w.LineCap = cap
	w.Set |= StateLineCap
	_, w.Err = fmt.Fprintln(w.Content, int(cap), "J")
}

// SetLineJoin sets the line join style.
//
// This implementes the PDF graphics operator "j".
func (w *Writer) SetLineJoin(join LineJoinStyle) {
	if !w.isValid("SetLineJoin", objPage|objText) {
		return
	}
	if LineJoinStyle(join) > 2 {
		join = 0
	}
	if w.isSet(StateLineJoin) && join == w.LineJoin {
		return
	}
	w.LineJoin = join
	w.Set |= StateLineJoin
	_, w.Err = fmt.Fprintln(w.Content, int(join), "j")
}

// SetMiterLimit sets the miter limit.
func (w *Writer) SetMiterLimit(limit float64) {
	if !w.isValid("SetMiterLimit", objPage|objText) {
		return
	}
	if w.isSet(StateMiterLimit) && nearlyEqual(limit, w.MiterLimit) {
		return
	}
	w.MiterLimit = limit
	w.Set |= StateMiterLimit
	_, w.Err = fmt.Fprintln(w.Content, float.Format(limit, 4), "M")
}

// SetDashPattern sets the line dash pattern.
func (w *Writer) SetDashPattern(pattern []float64, phase float64) {
	if !w.isValid("SetDashPattern", objPage|objText) {
		return
	}

	if w.isSet(StateDash) &&
		sliceNearlyEqual(pattern, w.DashPattern) &&
		nearlyEqual(phase, w.DashPhase) {
		return
	}
	w.DashPattern = pattern
	w.DashPhase = phase
	w.Set |= StateDash

	_, w.Err = fmt.Fprint(w.Content, "[")
	if w.Err != nil {
		return
	}
	sep := ""
	for _, x := range pattern {
		_, w.Err = fmt.Fprint(w.Content, sep, float.Format(x, 3))
		if w.Err != nil {
			return
		}
		sep = " "
	}
	_, w.Err = fmt.Fprint(w.Content, "] ", float.Format(phase, 3), " d\n")
}

// SetRenderingIntent sets the rendering intent.
func (w *Writer) SetRenderingIntent(intent pdf.Name) {
	if !w.isValid("SetRenderingIntent", objPage|objText) {
		return
	}
	if w.isSet(StateRenderingIntent) && intent == w.RenderingIntent {
		return
	}
	w.RenderingIntent = intent
	w.Set |= StateRenderingIntent
	err := intent.PDF(w.Content)
	if err != nil {
		w.Err = err
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " ri")
}

// SetFlatnessTolerance sets the flatness tolerance.
func (w *Writer) SetFlatnessTolerance(flatness float64) {
	if !w.isValid("SetFlatness", objPage|objText) {
		return
	}
	if w.isSet(StateFlatnessTolerance) && nearlyEqual(flatness, w.FlatnessTolerance) {
		return
	}
	w.FlatnessTolerance = flatness
	w.Set |= StateFlatnessTolerance
	_, w.Err = fmt.Fprintln(w.Content, float.Format(flatness, 3), "i")
}

// SetStrokeColor sets the stroke color in the graphics state.
// If col is nil, the stroke color is not changed.
func (w *Writer) SetStrokeColor(col color.Color) {
	if !w.isValid("SetStrokeColor", objPage|objText) {
		return
	}
	if w.isSet(StateStrokeColor) && col == w.StrokeColor {
		return
	}
	w.StrokeColor = col
	w.Set |= StateStrokeColor
	w.Err = col.SetStroke(w.Content)
}

// SetFillColor sets the fill color in the graphics state.
// If col is nil, the fill color is not changed.
func (w *Writer) SetFillColor(col color.Color) {
	if !w.isValid("SetFillColor", objPage|objText) {
		return
	}
	if w.isSet(StateFillColor) && col == w.FillColor {
		return
	}
	w.FillColor = col
	w.Set |= StateFillColor
	w.Err = col.SetFill(w.Content)
}

// GetResourceName returns the name of a resource.
// A new name is generated, if necessary, and the resource is added to the
// resource dictionary for the category.
func (w *Writer) getResourceName(category resourceCategory, r Resource) pdf.Name {
	name, ok := w.resName[catRes{category, r}]
	if ok {
		return name
	}

	var field *pdf.Dict
	var tmpl string
	switch category {
	case catFont:
		field = &w.Resources.Font
		tmpl = "F%d"
	case catExtGState:
		field = &w.Resources.ExtGState
		tmpl = "E%d"
	case catXObject:
		field = &w.Resources.XObject
		tmpl = "X%d"
	case catColorSpace:
		field = &w.Resources.ColorSpace
		tmpl = "C%d"
	case catPattern:
		field = &w.Resources.Pattern
		tmpl = "P%d"
	case catShading:
		field = &w.Resources.Shading
		tmpl = "S%d"
	case catProperties:
		field = &w.Resources.Properties
		tmpl = "MC%d"
	default:
		panic("invalid resource category " + category)
	}

	isUsed := func(name pdf.Name) bool {
		_, isUsed := w.nameUsed[catName{category, name}]
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
		for item := range w.nameUsed {
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
	w.resName[catRes{category, r}] = name
	w.nameUsed[catName{category, name}] = struct{}{}

	if *field == nil {
		*field = pdf.Dict{}
	}
	(*field)[name] = r.PDFObject()

	return name
}

// isValid returns true, if the current graphics object is one of the given types
// and if p.Err is nil.  Otherwise it sets p.Err and returns false.
func (w *Writer) isValid(cmd string, ss objectType) bool {
	if w.Err != nil {
		return false
	}

	if w.currentObject&ss != 0 {
		return true
	}

	w.Err = fmt.Errorf("unexpected state %q for %q", w.currentObject, cmd)
	return false
}

func (w *Writer) coord(x float64) string {
	// TODO(voss): Think about this some more.  Once we track the current
	// transformation matrix, we can use this to determine the number of digits
	// to keep.
	return float.Format(x, 2)
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

// See Figure 9 (p. 113) of PDF 32000-1:2008.
type objectType int

const (
	objPage objectType = 1 << iota
	objPath
	objText
	objClippingPath
	objInlineImage
	// objShading
	// objExternal
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
	case objInlineImage:
		return "inline image"
	default:
		return fmt.Sprintf("objectType(%d)", s)
	}
}

// Res represents a named PDF resource.
type Res struct {
	DefName pdf.Name
	Ref     pdf.Reference // TODO(voss): can this be pdf.Object?
}

// DefaultName implements the [Resource] interface.
func (r Res) DefaultName() pdf.Name {
	return r.DefName
}

// PDFObject implements the [Resource] interface.
func (r Res) PDFObject() pdf.Object {
	return r.Ref
}

// Resource represents the different PDF Resource types.
// Implementations of this must be "comparable".
type Resource interface {
	DefaultName() pdf.Name // return "" to choose names automatically
	PDFObject() pdf.Object // value to use in the resource dictionary
}
