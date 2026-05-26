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

// WindowType specifies the type of window that a media object plays in.  The
// zero value [WindowAnnotation] is the default.
type WindowType int

// Valid values for the [WindowType] type.
const (
	WindowAnnotation WindowType = iota // the screen annotation rectangle
	WindowFloating                     // a floating window
	WindowFullScreen                   // a full-screen window
	WindowHidden                       // a hidden window
)

func (w WindowType) toPDF() (pdf.Integer, bool) {
	switch w {
	case WindowFloating:
		return 0, true
	case WindowFullScreen:
		return 1, true
	case WindowHidden:
		return 2, true
	default:
		return 0, false // WindowAnnotation (the default) is omitted
	}
}

func windowTypeFromPDF(v pdf.Integer) WindowType {
	switch v {
	case 0:
		return WindowFloating
	case 1:
		return WindowFullScreen
	case 2:
		return WindowHidden
	default:
		return WindowAnnotation
	}
}

func (w WindowType) isValid() bool {
	return w >= WindowAnnotation && w <= WindowHidden
}

// MediaScreenParameters specifies where a media object should be played.
type MediaScreenParameters struct {
	// MustHonour holds the parameters that must be honoured for the media
	// screen parameters to be viable.
	MustHonour *MediaScreenEntries

	// BestEffort holds the parameters that should be honoured on a best-effort
	// basis.
	BestEffort *MediaScreenEntries

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// MediaScreenEntries holds the entries of a media screen parameters MH or BE
// dictionary.
type MediaScreenEntries struct {
	// Window is the type of window the media plays in.
	Window WindowType

	// Background (optional), if non-nil, is the DeviceRGB background colour of
	// the play rectangle, as three numbers in the range 0.0 to 1.0.
	Background []float64

	// Opacity, if set, is the opacity of the background colour, in the range
	// 0.0 to 1.0.
	Opacity optional.Float64

	// Monitor specifies which monitor a floating or full-screen window appears
	// on.
	Monitor MonitorSpecifier

	// FloatingWindow (optional) specifies the floating window.  It is required
	// when Window is [WindowFloating].
	FloatingWindow *FloatingWindowParameters
}

func (s *MediaScreenEntries) isEmpty() bool {
	if s == nil {
		return true
	}
	_, opacitySet := s.Opacity.Get()
	return s.Window == WindowAnnotation && s.Background == nil &&
		!opacitySet && s.Monitor == MonitorLargestDocument && s.FloatingWindow == nil
}

// ExtractMediaScreenParameters reads a media screen parameters dictionary.
func ExtractMediaScreenParameters(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*MediaScreenParameters, error) {
	dict, err := x.GetDictTyped(path, obj, "MediaScreenParams")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media screen parameters dictionary")
	}

	s := &MediaScreenParameters{SingleUse: isDirect}
	if s.MustHonour, err = extractScreenEntries(x, path, dict["MH"]); err != nil {
		return nil, err
	}
	if s.BestEffort, err = extractScreenEntries(x, path, dict["BE"]); err != nil {
		return nil, err
	}

	return s, nil
}

func extractScreenEntries(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) (*MediaScreenEntries, error) {
	dict, err := pdf.Optional(x.GetDict(path, obj))
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	s := &MediaScreenEntries{}

	if dict["W"] != nil {
		if w, err := pdf.Optional(x.GetInteger(path, dict["W"])); err != nil {
			return nil, err
		} else {
			s.Window = windowTypeFromPDF(w)
		}
	}
	if b, err := pdf.Optional(pdf.GetFloatArray(x.R, dict["B"])); err != nil {
		return nil, err
	} else if len(b) == 3 && inUnitRange(b) {
		s.Background = b
	}
	if dict["O"] != nil {
		if o, err := pdf.Optional(x.GetNumber(path, dict["O"])); err != nil {
			return nil, err
		} else if o >= 0 && o <= 1 {
			s.Opacity.Set(float64(o))
		}
	}
	if m, err := pdf.Optional(x.GetInteger(path, dict["M"])); err != nil {
		return nil, err
	} else if MonitorSpecifier(m).isValid() {
		s.Monitor = MonitorSpecifier(m)
	}
	if f, err := pdf.Optional(pdf.ExtractorGet(x, path, dict["F"], ExtractFloatingWindowParameters)); err != nil {
		return nil, err
	} else {
		s.FloatingWindow = f
	}

	if s.isEmpty() {
		return nil, nil
	}
	return s, nil
}

func inUnitRange(v []float64) bool {
	for _, x := range v {
		if x < 0 || x > 1 {
			return false
		}
	}
	return true
}

// Embed converts the media screen parameters to its PDF representation.
func (s *MediaScreenParameters) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media screen parameters", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaScreenParams")
	}

	if !s.MustHonour.isEmpty() {
		mh, err := s.MustHonour.toDict(e)
		if err != nil {
			return nil, err
		}
		dict["MH"] = mh
	}
	if !s.BestEffort.isEmpty() {
		be, err := s.BestEffort.toDict(e)
		if err != nil {
			return nil, err
		}
		dict["BE"] = be
	}

	if s.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

func (s *MediaScreenEntries) toDict(e *pdf.EmbedHelper) (pdf.Dict, error) {
	if !s.Window.isValid() {
		return nil, pdf.Error("media screen parameters: invalid Window")
	}
	if !s.Monitor.isValid() {
		return nil, pdf.Error("media screen parameters: invalid Monitor")
	}
	if s.Background != nil {
		if len(s.Background) != 3 || !inUnitRange(s.Background) {
			return nil, pdf.Error("media screen parameters: Background must be three values in 0.0..1.0")
		}
	}
	if s.Window == WindowFloating && s.FloatingWindow == nil {
		return nil, pdf.Error("media screen parameters: FloatingWindow is required for a floating window")
	}

	dict := pdf.Dict{}
	if w, ok := s.Window.toPDF(); ok {
		dict["W"] = w
	}
	if s.Background != nil {
		arr := make(pdf.Array, 3)
		for i, c := range s.Background {
			arr[i] = pdf.Number(c)
		}
		dict["B"] = arr
	}
	if o, ok := s.Opacity.Get(); ok {
		if o < 0 || o > 1 {
			return nil, pdf.Error("media screen parameters: Opacity must be in 0.0..1.0")
		}
		dict["O"] = pdf.Number(o)
	}
	if s.Monitor != MonitorLargestDocument {
		dict["M"] = pdf.Integer(s.Monitor)
	}
	if s.FloatingWindow != nil {
		f, err := e.Embed(s.FloatingWindow)
		if err != nil {
			return nil, err
		}
		dict["F"] = f
	}

	return dict, nil
}
