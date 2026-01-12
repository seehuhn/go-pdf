// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pdf

// This file contains more complex PDF data structures, which are composed
// of the elementary types from "objects.go".

import (
	"bytes"
	"fmt"
	"iter"
	"math"
	"strings"
	"time"
	"unicode/utf16"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
)

// A Number is either an Integer or a Real.
type Number float64

// GetNumber is a helper function for reading numeric values from a PDF file.
// This resolves indirect references and makes sure the resulting object is an
// Integer or a Real.
//
// TODO(voss): should this return float64?
func GetNumber(r Getter, obj Object) (Number, error) {
	obj, err := Resolve(r, obj)
	if err != nil {
		return 0, err
	}
	switch x := obj.(type) {
	case Integer:
		return Number(x), nil
	case Real:
		return Number(x), nil
	case nil:
		return 0, nil
	default:
		return 0, &MalformedFileError{
			Err: fmt.Errorf("expected Number but got %T", obj),
		}
	}
}

// AsPDF implements the [Object] interface.
func (x Number) AsPDF(opt OutputOptions) Native {
	var obj Native
	if i := Integer(x); Number(i) == x {
		obj = i
	} else {
		obj = Real(x)
	}
	return obj
}

type TextString string

// GetTextString interprets x as a PDF "text string" and returns
// the corresponding utf-8 encoded string.
func GetTextString(r Getter, obj Object) (TextString, error) {
	s, err := GetString(r, obj)
	if err != nil {
		return "", err
	}
	return s.AsTextString(), nil
}

var utf16Marker = []byte{254, 255}
var utf8Marker = []byte{239, 187, 191}

func (s TextString) AsPDF(opt OutputOptions) Native {
	// use PDFDocEncoding where possible, because it is smallest
	if buf, ok := PDFDocEncode(string(s)); ok && !bytes.HasPrefix(buf, utf16Marker) && !bytes.HasPrefix(buf, utf8Marker) {
		return buf
	}

	// Otherwise, use UTF-8 if supported.
	if opt.HasAny(OptTextStringUtf8) {
		obj := make(String, 0, 3+len(s))
		obj = append(obj, 239, 187, 191)
		obj = append(obj, []byte(s)...)
		return obj
	}

	// Otherwise, use UTF-16.
	var buf = []uint16{0xFEFF}
	for _, r := range s {
		buf = utf16.AppendRune(buf, r)
	}
	out := make(String, 0, 2*len(buf)+2)
	for _, x := range buf {
		out = append(out, byte(x>>8), byte(x))
	}
	return out
}

func (x String) AsTextString() TextString {
	b := []byte(x)

	var s string
	if bytes.HasPrefix(b, utf16Marker) {
		buf := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			buf = append(buf, uint16(b[i])<<8|uint16(b[i+1]))
		}
		rr := utf16.Decode(buf)
		s = string(rr)
	} else if bytes.HasPrefix(b, utf8Marker) {
		s = string(b[3:])
	} else {
		s = PDFDocDecode(x)
	}

	return TextString(s)
}

func (s TextString) AsTextString() TextString {
	return s
}

func (x Name) AsTextString() TextString {
	return TextString(x)
}

type asTextStringer interface {
	AsTextString() TextString
}

type Date time.Time

// Now returns the current date and time as a Date object.
func Now() Date {
	return Date(time.Now())
}

func (d Date) String() string {
	return time.Time(d).Format(time.RFC3339)
}

func (d Date) IsZero() bool {
	return time.Time(d).IsZero()
}

func (d Date) Equal(other Date) bool {
	return time.Time(d).Equal(time.Time(other))
}

func GetDate(r Getter, obj Object) (Date, error) {
	var zero Date

	s, err := GetString(r, obj)
	if err != nil {
		return zero, err
	}
	return s.AsDate()
}

// AsPDF creates a PDF String object encoding the given date and time.
func (d Date) AsPDF(opt OutputOptions) Native {
	s := time.Time(d).Format("D:20060102150405-0700")
	k := len(s) - 2
	s = s[:k] + "'" + s[k:]
	return String(s)
}

// TODO(voss): remove
func (d Date) AsDate() (Date, error) {
	return d, nil
}

// AsDate converts a PDF date string to a Date object.
// If the string does not have the correct format, an error is returned.
func (x String) AsDate() (Date, error) {
	var zero Date

	s := string(x.AsTextString())

	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "'", "")
	if s == "D:" || s == "" {
		return zero, nil
	}
	if strings.HasPrefix(s, "19") || strings.HasPrefix(s, "20") {
		s = "D:" + s
	}

	formats := []string{
		"D:20060102150405-0700",
		"D:20060102150405-07",
		"D:20060102150405Z0000",
		"D:20060102150405Z00",
		"D:20060102150405Z",
		"D:20060102150405",
		"D:200601021504-0700",
		"D:200601021504-07",
		"D:200601021504Z0000",
		"D:200601021504Z00",
		"D:200601021504Z",
		"D:200601021504",
		"D:2006010215",
		"D:20060102",
		"D:200601",
		"D:2006",
		time.ANSIC,
	}
	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			// truncate fractional seconds for consistency with PDF format
			t = t.Truncate(time.Second)
			return Date(t), nil
		}
	}
	return zero, errNoDate
}

// TODO(voss): remove
type asDater interface {
	AsDate() (Date, error)
}

// Rectangle represents a PDF rectangle.
type Rectangle struct {
	LLx, LLy, URx, URy float64
}

// Dx returns the width of the rectangle.
func (r *Rectangle) Dx() float64 {
	return r.URx - r.LLx
}

// Dy returns the height of the rectangle.
func (r *Rectangle) Dy() float64 {
	return r.URy - r.LLy
}

// IsDirect returns true.  This makes the [IsDirect] function work for
// Rectangle objects.
func (r *Rectangle) IsDirect() bool {
	return true
}

// GetRectangle resolves references to indirect objects and makes sure the
// resulting object is a PDF rectangle object.
// If the object is null, nil is returned.
func GetRectangle(r Getter, obj Object) (*Rectangle, error) {
	if rect, ok := obj.(*Rectangle); ok {
		return rect, nil
	}

	a, err := GetArray(r, obj)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil
	}

	return asRectangle(r, a)
}

// asRectangle converts an array of 4 numbers to a Rectangle object.
// If the array does not have the correct format, an error is returned.
func asRectangle(r Getter, a Array) (*Rectangle, error) {
	if len(a) != 4 {
		return nil, errNoRectangle
	}
	values, err := GetFloatArray(r, a)
	if err != nil {
		return nil, err
	}
	if len(values) != 4 {
		return nil, errNoRectangle
	}
	rect := &Rectangle{
		LLx: math.Min(values[0], values[2]),
		LLy: math.Min(values[1], values[3]),
		URx: math.Max(values[0], values[2]),
		URy: math.Max(values[1], values[3]),
	}
	return rect, nil
}

func (r *Rectangle) String() string {
	return fmt.Sprintf("[%.2f %.2f %.2f %.2f]", r.LLx, r.LLy, r.URx, r.URy)
}

func (r *Rectangle) AsPDF(opt OutputOptions) Native {
	res := make(Array, 4)
	for i, x := range []float64{r.LLx, r.LLy, r.URx, r.URy} {
		res[i] = Number(x).AsPDF(opt)
	}
	return res
}

// IsZero is true if the rectangle is the zero rectangle object.
func (r Rectangle) IsZero() bool {
	return r.LLx == 0 && r.LLy == 0 && r.URx == 0 && r.URy == 0
}

// Equal reports whether two rectangles have identical coordinates.
func (r *Rectangle) Equal(other *Rectangle) bool {
	if r == nil || other == nil {
		return r == other
	}
	return r.LLx == other.LLx && r.LLy == other.LLy &&
		r.URx == other.URx && r.URy == other.URy
}

// NearlyEqual reports whether the corner coordinates of two rectangles
// differ by less than `eps`.
func (r *Rectangle) NearlyEqual(other *Rectangle, eps float64) bool {
	return (math.Abs(r.LLx-other.LLx) < eps &&
		math.Abs(r.LLy-other.LLy) < eps &&
		math.Abs(r.URx-other.URx) < eps &&
		math.Abs(r.URy-other.URy) < eps)
}

// XPos returns the x-coordinate of a point in the rectangle, given a relative
// position between 0 and 1.  The relative position 0 corresponds to the left
// edge of the rectangle, and 1 to the right edge.
func (r *Rectangle) XPos(rel float64) float64 {
	return r.LLx + rel*(r.URx-r.LLx)
}

// YPos returns the y-coordinate of a point in the rectangle, given a relative
// position between 0 and 1.  The relative position 0 corresponds to the bottom
// edge of the rectangle, and 1 to the top edge.
func (r *Rectangle) YPos(rel float64) float64 {
	return r.LLy + rel*(r.URy-r.LLy)
}

// Extend enlarges the rectangle to also cover `other`.
func (r *Rectangle) Extend(other *Rectangle) {
	if other.IsZero() {
		return
	}
	if r.IsZero() {
		*r = *other
		return
	}
	if other.LLx < r.LLx {
		r.LLx = other.LLx
	}
	if other.LLy < r.LLy {
		r.LLy = other.LLy
	}
	if other.URx > r.URx {
		r.URx = other.URx
	}
	if other.URy > r.URy {
		r.URy = other.URy
	}
}

// ExtendVec enlarges the rectangle to also cover v.
func (r *Rectangle) ExtendVec(v vec.Vec2) {
	isZero := r.IsZero()
	if v.X < r.LLx || isZero {
		r.LLx = v.X
	}
	if v.Y < r.LLy || isZero {
		r.LLy = v.Y
	}
	if v.X > r.URx || isZero {
		r.URx = v.X
	}
	if v.Y > r.URy || isZero {
		r.URy = v.Y
	}
}

// Round rounds the corner coordinates of the rectangle to the given number of
// decimal places and returns the result.
func (r Rectangle) Round(digits int) Rectangle {
	return Rectangle{
		LLx: Round(r.LLx, digits),
		LLy: Round(r.LLy, digits),
		URx: Round(r.URx, digits),
		URy: Round(r.URy, digits),
	}
}

// IRound rounds the corner coordinates of the rectangle to the given number of
// decimal places. This modifies the rectangle in place.
func (r *Rectangle) IRound(digits int) {
	r.LLx = Round(r.LLx, digits)
	r.LLy = Round(r.LLy, digits)
	r.URx = Round(r.URx, digits)
	r.URy = Round(r.URy, digits)
}

// Contains checks if a point is within the rectangle.
// The rectangle coordinates are in normalized form (lower-left, upper-right).
func (r *Rectangle) Contains(point vec.Vec2) bool {
	return point.X >= r.LLx && point.X <= r.URx &&
		point.Y >= r.LLy && point.Y <= r.URy
}

func GetMatrix(r Getter, obj Object) (m matrix.Matrix, err error) {
	defer func() {
		if err != nil {
			err = Wrap(err, "GetMatrix")
		}
	}()

	a, err := GetFloatArray(r, obj)
	if err != nil {
		return matrix.Matrix{}, err
	}

	if len(a) != 6 {
		return m, &MalformedFileError{
			Err: fmt.Errorf("expected 6 numbers, got %d", len(a)),
		}
	}

	copy(m[:], a)

	return m, nil
}

// Info represents a PDF Document Information Dictionary.
// All fields in this structure are optional.
//
// The Document Information Dictionary is documented in section
// 14.3.3 of PDF 32000-1:2008.
type Info struct {
	Title    TextString `pdf:"optional"`
	Author   TextString `pdf:"optional"`
	Subject  TextString `pdf:"optional"`
	Keywords TextString `pdf:"optional"`

	// Creator gives the name of the application that created the original
	// document, if the document was converted to PDF from another format.
	Creator TextString `pdf:"optional"`

	// Producer gives the name of the application that converted the document,
	// if the document was converted to PDF from another format.
	Producer TextString `pdf:"optional"`

	// CreationDate gives the date and time the document was created.
	CreationDate Date `pdf:"optional"`

	// ModDate gives the date and time the document was most recently modified.
	ModDate Date `pdf:"optional"`

	// Trapped indicates whether the document has been modified to include
	// trapping information.  (A trap is an overlap between adjacent areas of
	// of different colours, used to avoid visual problems caused by imprecise
	// alignment of different layers of ink.) Possible values are:
	//   * "True": The document has been fully trapped.  No further trapping is
	//     necessary.
	//   * "False": The document has not been trapped.
	//   * "Unknown" (default): Either it is unknown whether the document has
	//     been trapped, or the document has been partially trapped.  Further
	//     trapping may be necessary.
	Trapped Name `pdf:"optional,allowstring"`

	// Custom contains all non-standard fields in the Info dictionary.
	Custom map[string]string `pdf:"extra"`
}

// Function represents a PDF function.
// Concrete implementations of this interface can be found in the
// seehuhn.de/go/pdf/function package.
type Function interface {
	// FunctionType returns the type of the PDF function.
	// This is one of 0, 2, 3, 4.
	FunctionType() int

	// Shape returns the number of input and output values of the function.
	Shape() (m int, n int)

	// GetDomain returns the function's input domain in array format
	// [min0, max0, min1, max1, ...] where each pair represents the valid
	// range for one input variable.
	GetDomain() []float64

	Embedder

	// Apply applies the function to the given m input values
	// and returns the n output values.
	Apply(...float64) []float64
}

// NumberTree represents a PDF number tree.
// A number tree is a dictionary that maps integer keys to values.
// Implementations of this interface can be found in the
// seehuhn.de/go/pdf/numtree package.
type NumberTree interface {
	// Lookup returns the value for a given key in the number tree.
	// If the key is not found, it returns nil and [ErrKeyNotFound].
	Lookup(key Integer) (Object, error)

	All() iter.Seq2[Integer, Object]

	Embedder
}

// NameTree represents a PDF name tree.
// A name tree is a dictionary that maps names to values.
// Implementations of this interface can be found in the
// seehuhn.de/go/pdf/nametree package.
type NameTree interface {
	// Lookup returns the value for a given key in the name tree.
	// If the key is not found, it returns nil and [ErrKeyNotFound].
	Lookup(key Name) (Object, error)

	All() iter.Seq2[Name, Object]

	Embedder
}

// Action represents a PDF action.
// The different implementations of this interface can be found in the
// seehuhn.de/go/pdf/action package.
type Action interface {
	// ActionType returns the type of the action, e.g. "GoTo", "URI", etc.
	ActionType() Name

	Next() []Object

	Object
}
