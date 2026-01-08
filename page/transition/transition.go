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

package transition

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// Style represents a transition style for page transitions.
type Style pdf.Name

// Transition styles.
const (
	StyleSplit    Style = "Split"    // two lines sweep across the screen
	StyleBlinds   Style = "Blinds"   // multiple lines sweep in the same direction
	StyleBox      Style = "Box"      // rectangular box sweeps in/out
	StyleWipe     Style = "Wipe"     // single line sweeps across
	StyleDissolve Style = "Dissolve" // gradual dissolve
	StyleGlitter  Style = "Glitter"  // dissolve with directional sweep
	StyleReplace  Style = "R"        // no effect (default)
	StyleFly      Style = "Fly"      // changes fly in/out (PDF 1.5)
	StylePush     Style = "Push"     // old page slides out, new slides in (PDF 1.5)
	StyleCover    Style = "Cover"    // new page slides in, covering old (PDF 1.5)
	StyleUncover  Style = "Uncover"  // old page slides out, uncovering new (PDF 1.5)
	StyleFade     Style = "Fade"     // new page gradually becomes visible (PDF 1.5)
)

// Dimension specifies whether a transition effect occurs horizontally or vertically.
// Used by Split and Blinds styles.
type Dimension pdf.Name

// Dimension values.
const (
	DimensionHorizontal Dimension = "H" // default
	DimensionVertical   Dimension = "V"
)

// Motion specifies whether a transition moves inward or outward.
// Used by Split, Box, and Fly styles.
type Motion pdf.Name

// Motion values.
const (
	MotionInward  Motion = "I" // from edges to center (default)
	MotionOutward Motion = "O" // from center to edges
)

// Direction specifies the direction of motion for a transition effect.
// Used by Wipe, Glitter, Fly, Cover, Uncover, and Push styles.
//
// Values are angles in degrees, measured counterclockwise from a
// left-to-right direction. Valid values depend on the transition style:
//   - Wipe: 0, 90, 180, 270
//   - Glitter: 0, 270, 315
//   - Fly, Cover, Uncover, Push: 0, 270
//
// The special value [DirNone] is valid only for [StyleFly] when Scale ≠ 1.0.
type Direction int

// DirNone represents the PDF name "None", valid only for Fly with Scale ≠ 1.0.
const DirNone Direction = -1

// PDF 2.0 sections: 12.4.4

// Transition represents a transition dictionary (Table 164 in the PDF spec).
//
// Transition dictionaries control the visual effect used when moving
// from another page to the page containing this transition during a
// presentation.
type Transition struct {
	// Style specifies the transition style.
	// Default: StyleReplace (no effect).
	Style Style

	// Duration is the duration of the transition effect in seconds.
	// Default: 1.0.
	Duration float64

	// Dimension specifies horizontal or vertical transition.
	// Only used by Split and Blinds styles.
	// Default: DimensionHorizontal.
	Dimension Dimension

	// Motion specifies inward or outward transition.
	// Only used by Split, Box, and Fly styles.
	// Default: MotionInward.
	Motion Motion

	// Direction specifies the direction of motion, in degrees counterclockwise
	// from left-to-right. Only used by Wipe, Glitter, Fly, Cover, Uncover, and
	// Push styles.
	Direction Direction

	// Scale is the starting or ending scale for Fly transitions.
	// Only used by Fly style (PDF 1.5).
	// Default: 1.0.
	Scale float64

	// Opaque specifies whether the flown area is rectangular and opaque.
	// Only used by Fly style (PDF 1.5).
	// Default: false.
	Opaque bool

	// SingleUse indicates that this transition is used only once.
	SingleUse bool
}

// Embed adds the transition dictionary to a PDF file.
//
// This implements the [pdf.Embedder] interface.
func (t *Transition) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "page transitions", pdf.V1_1); err != nil {
		return nil, err
	}

	// check for PDF 1.5 features
	if t.Style == StyleFly || t.Style == StylePush || t.Style == StyleCover ||
		t.Style == StyleUncover || t.Style == StyleFade {
		if err := pdf.CheckVersion(rm.Out(), "transition style "+string(t.Style), pdf.V1_5); err != nil {
			return nil, err
		}
	}

	dict := pdf.Dict{}

	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Trans")
	}

	// S: style (default R)
	if t.Style != "" && t.Style != StyleReplace {
		dict["S"] = pdf.Name(t.Style)
	}

	// D: duration (default 1.0)
	if t.Duration != 0 && t.Duration != 1.0 {
		dict["D"] = pdf.Number(t.Duration)
	}

	// Dm: dimension (default H) - Split and Blinds only
	if t.Dimension != "" && t.Dimension != DimensionHorizontal {
		dict["Dm"] = pdf.Name(t.Dimension)
	}

	// M: motion (default I) - Split, Box, Fly only
	if t.Motion != "" && t.Motion != MotionInward {
		dict["M"] = pdf.Name(t.Motion)
	}

	// Di: direction (default 0)
	if t.Direction == DirNone {
		dict["Di"] = pdf.Name("None")
	} else if t.Direction < 0 {
		return nil, fmt.Errorf("invalid transition direction %d", t.Direction)
	} else if t.Direction != 0 {
		dict["Di"] = pdf.Integer(t.Direction)
	}

	// SS: scale (default 1.0) - Fly only, PDF 1.5
	if t.Scale != 0 && t.Scale != 1.0 {
		if err := pdf.CheckVersion(rm.Out(), "transition SS", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["SS"] = pdf.Number(t.Scale)
	}

	// B: opaque (default false) - Fly only, PDF 1.5
	if t.Opaque {
		if err := pdf.CheckVersion(rm.Out(), "transition B", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["B"] = pdf.Boolean(true)
	}

	if t.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	if err := rm.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// Extract reads a transition dictionary from a PDF file.
func Extract(x *pdf.Extractor, obj pdf.Object) (*Transition, error) {
	_, isIndirect := obj.(pdf.Reference)

	dict, err := x.GetDictTyped(obj, "Trans")
	if err != nil {
		return nil, err
	}

	t := &Transition{}

	// S: style
	if s, err := pdf.Optional(x.GetName(dict["S"])); err != nil {
		return nil, err
	} else if s != "" {
		t.Style = Style(s)
	}

	// D: duration
	if d, err := pdf.Optional(x.GetNumber(dict["D"])); err != nil {
		return nil, err
	} else if d != 0 {
		t.Duration = d
	}

	// Dm: dimension
	if dm, err := pdf.Optional(x.GetName(dict["Dm"])); err != nil {
		return nil, err
	} else if dm != "" {
		t.Dimension = Dimension(dm)
	}

	// M: motion
	if m, err := pdf.Optional(x.GetName(dict["M"])); err != nil {
		return nil, err
	} else if m != "" {
		t.Motion = Motion(m)
	}

	// Di: direction (can be integer or name "None")
	if diObj := dict["Di"]; diObj != nil {
		resolved, err := x.Resolve(diObj)
		if err != nil {
			return nil, err
		}
		switch di := resolved.(type) {
		case pdf.Name:
			if di == "None" {
				t.Direction = DirNone
			}
		case pdf.Integer:
			if di >= 0 {
				t.Direction = Direction(di)
			}
		}
	}

	// SS: scale
	if ss, err := pdf.Optional(x.GetNumber(dict["SS"])); err != nil {
		return nil, err
	} else if ss != 0 {
		t.Scale = ss
	}

	// B: opaque
	if b, err := pdf.Optional(x.GetBoolean(dict["B"])); err != nil {
		return nil, err
	} else {
		t.Opaque = bool(b)
	}

	t.SingleUse = !isIndirect

	return t, nil
}
