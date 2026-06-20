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

package media

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/optional"
)

// WindowRelativeTo specifies the window relative to which a floating window is
// positioned.  The zero value [RelativeToDocument] is the default.
type WindowRelativeTo int

// Valid values for the [WindowRelativeTo] type.
const (
	RelativeToDocument    WindowRelativeTo = iota // the document window
	RelativeToApplication                         // the application window
	RelativeToDesktop                             // the full virtual desktop
	RelativeToMonitor                             // the monitor specified by Monitor
)

func (r WindowRelativeTo) isValid() bool {
	return r >= RelativeToDocument && r <= RelativeToMonitor
}

// WindowPosition specifies where a floating window is positioned relative to
// its reference window.  The zero value [PositionCentre] is the default.
type WindowPosition int

// Valid values for the [WindowPosition] type.
const (
	PositionCentre WindowPosition = iota
	PositionUpperLeft
	PositionUpperCentre
	PositionUpperRight
	PositionCentreLeft
	PositionCentreRight
	PositionLowerLeft
	PositionLowerCentre
	PositionLowerRight
)

func (p WindowPosition) toPDF() (pdf.Integer, bool) {
	switch p {
	case PositionUpperLeft:
		return 0, true
	case PositionUpperCentre:
		return 1, true
	case PositionUpperRight:
		return 2, true
	case PositionCentreLeft:
		return 3, true
	case PositionCentreRight:
		return 5, true
	case PositionLowerLeft:
		return 6, true
	case PositionLowerCentre:
		return 7, true
	case PositionLowerRight:
		return 8, true
	default:
		return 0, false // PositionCentre (the default) is omitted
	}
}

func windowPositionFromPDF(v pdf.Integer) WindowPosition {
	switch v {
	case 0:
		return PositionUpperLeft
	case 1:
		return PositionUpperCentre
	case 2:
		return PositionUpperRight
	case 3:
		return PositionCentreLeft
	case 5:
		return PositionCentreRight
	case 6:
		return PositionLowerLeft
	case 7:
		return PositionLowerCentre
	case 8:
		return PositionLowerRight
	default:
		return PositionCentre
	}
}

func (p WindowPosition) isValid() bool {
	return p >= PositionCentre && p <= PositionLowerRight
}

// OffscreenAction specifies what happens if a floating window is positioned
// off-screen.  The zero value [OffscreenMoveOnscreen] is the default.
type OffscreenAction int

// Valid values for the [OffscreenAction] type.
const (
	OffscreenMoveOnscreen OffscreenAction = iota // move or resize the window on-screen
	OffscreenNoAction                            // take no special action
	OffscreenNonViable                           // consider the object non-viable
)

func (o OffscreenAction) toPDF() (pdf.Integer, bool) {
	switch o {
	case OffscreenNoAction:
		return 0, true
	case OffscreenNonViable:
		return 2, true
	default:
		return 0, false // OffscreenMoveOnscreen (the default) is omitted
	}
}

func offscreenActionFromPDF(v pdf.Integer) OffscreenAction {
	switch v {
	case 0:
		return OffscreenNoAction
	case 2:
		return OffscreenNonViable
	default:
		return OffscreenMoveOnscreen
	}
}

func (o OffscreenAction) isValid() bool {
	return o >= OffscreenMoveOnscreen && o <= OffscreenNonViable
}

// Resizable specifies whether a floating window may be resized by the user.
// The zero value [ResizeNone] is the default.
type Resizable int

// Valid values for the [Resizable] type.
const (
	ResizeNone       Resizable = iota // may not be resized
	ResizeKeepAspect                  // may be resized only if the aspect ratio is preserved
	ResizeFree                        // may be resized freely
)

func (r Resizable) isValid() bool {
	return r >= ResizeNone && r <= ResizeFree
}

// FloatingWindowParameters specifies the size, position and options used in
// displaying a floating window.
type FloatingWindowParameters struct {
	// Width is the floating window's width in pixels.
	Width int

	// Height is the floating window's height in pixels.
	Height int

	// RelativeTo is the window relative to which the floating window is
	// positioned.
	RelativeTo WindowRelativeTo

	// Position is where the floating window is positioned relative to its
	// reference window.
	Position WindowPosition

	// Offscreen specifies what happens if the floating window is positioned
	// off-screen.
	Offscreen OffscreenAction

	// TitleBar, if set, controls whether the floating window has a title bar.
	TitleBar optional.Bool

	// UserCanClose, if set, controls whether the window includes a close
	// control.  Meaningful only if TitleBar is true.
	UserCanClose optional.Bool

	// Resizable specifies whether the user may resize the window.
	Resizable Resizable

	// Title (optional) is the text displayed on the title bar.  Meaningful
	// only if TitleBar is true.
	Title MultiLangText

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractFloatingWindowParameters reads a floating window parameters
// dictionary.
func ExtractFloatingWindowParameters(c pdf.Cursor, obj pdf.Object, isDirect bool) (*FloatingWindowParameters, error) {
	dict, err := c.DictTyped(obj, "FWParams")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing floating window parameters dictionary")
	}

	arr, err := pdf.Optional(c.Array(dict["D"]))
	if err != nil {
		return nil, err
	} else if len(arr) < 2 {
		return nil, pdf.Error("invalid floating window dimensions")
	}
	w, errW := pdf.Optional(c.Integer(arr[0]))
	h, errH := pdf.Optional(c.Integer(arr[1]))
	if errW != nil {
		return nil, errW
	}
	if errH != nil {
		return nil, errH
	}
	if w < 0 || h < 0 {
		return nil, pdf.Error("invalid floating window dimensions")
	}

	f := &FloatingWindowParameters{
		Width:     int(w),
		Height:    int(h),
		SingleUse: isDirect,
	}

	if rt, err := pdf.Optional(c.Integer(dict["RT"])); err != nil {
		return nil, err
	} else if WindowRelativeTo(rt).isValid() {
		f.RelativeTo = WindowRelativeTo(rt)
	}
	if dict["P"] != nil {
		if p, err := pdf.Optional(c.Integer(dict["P"])); err != nil {
			return nil, err
		} else {
			f.Position = windowPositionFromPDF(p)
		}
	}
	if dict["O"] != nil {
		if o, err := pdf.Optional(c.Integer(dict["O"])); err != nil {
			return nil, err
		} else {
			f.Offscreen = offscreenActionFromPDF(o)
		}
	}
	if dict["T"] != nil {
		if t, err := pdf.Optional(c.Boolean(dict["T"])); err != nil {
			return nil, err
		} else {
			f.TitleBar.Set(bool(t))
		}
	}
	if dict["UC"] != nil {
		if uc, err := pdf.Optional(c.Boolean(dict["UC"])); err != nil {
			return nil, err
		} else {
			f.UserCanClose.Set(bool(uc))
		}
	}
	if r, err := pdf.Optional(c.Integer(dict["R"])); err != nil {
		return nil, err
	} else if Resizable(r).isValid() {
		f.Resizable = Resizable(r)
	}
	if tt, err := extractMultiLangText(c, dict["TT"]); err != nil {
		return nil, err
	} else {
		f.Title = tt
	}

	return f, nil
}

// Embed converts the floating window parameters to its PDF representation.
func (f *FloatingWindowParameters) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "floating window parameters", pdf.V1_5); err != nil {
		return nil, err
	}
	if f.Width < 0 || f.Height < 0 {
		return nil, pdf.Error("floating window parameters: dimensions must not be negative")
	}
	if !f.RelativeTo.isValid() {
		return nil, pdf.Error("floating window parameters: invalid RelativeTo")
	}
	if !f.Position.isValid() {
		return nil, pdf.Error("floating window parameters: invalid Position")
	}
	if !f.Offscreen.isValid() {
		return nil, pdf.Error("floating window parameters: invalid Offscreen")
	}
	if !f.Resizable.isValid() {
		return nil, pdf.Error("floating window parameters: invalid Resizable")
	}

	dict := pdf.Dict{
		"D": pdf.Array{pdf.Integer(f.Width), pdf.Integer(f.Height)},
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("FWParams")
	}
	if f.RelativeTo != RelativeToDocument {
		dict["RT"] = pdf.Integer(f.RelativeTo)
	}
	if p, ok := f.Position.toPDF(); ok {
		dict["P"] = p
	}
	if o, ok := f.Offscreen.toPDF(); ok {
		dict["O"] = o
	}
	if t, ok := f.TitleBar.Get(); ok {
		dict["T"] = pdf.Boolean(t)
	}
	if uc, ok := f.UserCanClose.Get(); ok {
		dict["UC"] = pdf.Boolean(uc)
	}
	if f.Resizable != ResizeNone {
		dict["R"] = pdf.Integer(f.Resizable)
	}
	if len(f.Title) > 0 {
		dict["TT"] = f.Title.toArray()
	}

	if f.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
