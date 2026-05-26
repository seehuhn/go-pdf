// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package appearance

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
)

// PDF 2.0 sections: 12.5.6.19

// TextPosition specifies where the caption of a button annotation is
// positioned relative to its icon.
type TextPosition int

const (
	TextPositionCaptionOnly        TextPosition = 0 // no icon; caption only
	TextPositionIconOnly           TextPosition = 1 // no caption; icon only
	TextPositionCaptionBelowIcon   TextPosition = 2 // caption below the icon
	TextPositionCaptionAboveIcon   TextPosition = 3 // caption above the icon
	TextPositionCaptionRightOfIcon TextPosition = 4 // caption to the right of the icon
	TextPositionCaptionLeftOfIcon  TextPosition = 5 // caption to the left of the icon
	TextPositionCaptionOverIcon    TextPosition = 6 // caption overlaid on the icon
)

// Characteristics is an appearance characteristics dictionary, holding
// information for constructing the dynamic appearance of a widget or screen
// annotation. It corresponds to the /MK entry of such annotations.
type Characteristics struct {
	// Rotation (optional) is the number of degrees by which the annotation is
	// rotated counterclockwise relative to the page. It is a multiple of 90.
	Rotation int

	// BorderColor (optional) is the colour of the annotation's border. It uses
	// the DeviceGray, DeviceRGB, or DeviceCMYK colour space.
	BorderColor color.Color

	// BackgroundColor (optional) is the colour of the annotation's background.
	// It uses the DeviceGray, DeviceRGB, or DeviceCMYK colour space.
	BackgroundColor color.Color

	// Caption (optional) is the annotation's normal caption, displayed when it
	// is not interacting with the user.
	Caption string

	// RolloverCaption (optional) is the caption displayed when the user rolls
	// the cursor into the annotation's active area without pressing the mouse
	// button.
	RolloverCaption string

	// DownCaption (optional) is the caption displayed when the mouse button is
	// pressed within the annotation's active area.
	DownCaption string

	// Icon (optional) is the annotation's normal icon, displayed when it is not
	// interacting with the user.
	Icon *form.Form

	// RolloverIcon (optional) is the icon displayed when the user rolls the
	// cursor into the annotation's active area without pressing the mouse
	// button.
	RolloverIcon *form.Form

	// DownIcon (optional) is the icon displayed when the mouse button is
	// pressed within the annotation's active area.
	DownIcon *form.Form

	// IconFit (optional) specifies how the annotation's icons are displayed
	// within the annotation rectangle.
	IconFit *IconFit

	// TextPosition (optional) indicates where to position the caption relative
	// to the icon. It is one of the TextPosition constants.
	TextPosition TextPosition

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder = (*Characteristics)(nil)

// ExtractCharacteristics reads an appearance characteristics dictionary from
// the PDF object obj. If obj is absent or resolves to null,
// ExtractCharacteristics returns (nil, nil).
func ExtractCharacteristics(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*Characteristics, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	res := &Characteristics{SingleUse: isDirect}

	if r, err := pdf.Optional(x.GetInteger(path, dict["R"])); err != nil {
		return nil, err
	} else if r%90 == 0 {
		res.Rotation = int(r)
	}

	if bc, err := pdf.Optional(colorenc.Extract(x.R, dict["BC"])); err != nil {
		return nil, err
	} else {
		res.BorderColor = bc
	}

	if bg, err := pdf.Optional(colorenc.Extract(x.R, dict["BG"])); err != nil {
		return nil, err
	} else {
		res.BackgroundColor = bg
	}

	if ca, err := pdf.Optional(pdf.GetTextString(x.R, dict["CA"])); err != nil {
		return nil, err
	} else {
		res.Caption = string(ca)
	}

	if rc, err := pdf.Optional(pdf.GetTextString(x.R, dict["RC"])); err != nil {
		return nil, err
	} else {
		res.RolloverCaption = string(rc)
	}

	if ac, err := pdf.Optional(pdf.GetTextString(x.R, dict["AC"])); err != nil {
		return nil, err
	} else {
		res.DownCaption = string(ac)
	}

	if f, err := pdf.ExtractorGetOptional(x, path, dict["I"], extract.Form); err != nil {
		return nil, err
	} else {
		res.Icon = f
	}

	if f, err := pdf.ExtractorGetOptional(x, path, dict["RI"], extract.Form); err != nil {
		return nil, err
	} else {
		res.RolloverIcon = f
	}

	if f, err := pdf.ExtractorGetOptional(x, path, dict["IX"], extract.Form); err != nil {
		return nil, err
	} else {
		res.DownIcon = f
	}

	if fit, err := pdf.ExtractorGetOptional(x, path, dict["IF"], ExtractIconFit); err != nil {
		return nil, err
	} else {
		res.IconFit = fit
	}

	if tp, err := pdf.Optional(x.GetInteger(path, dict["TP"])); err != nil {
		return nil, err
	} else if tp >= 0 && tp <= 6 {
		res.TextPosition = TextPosition(tp)
	}

	return res, nil
}

func (c *Characteristics) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "appearance characteristics dictionary", pdf.V1_2); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}

	if c.Rotation != 0 {
		if c.Rotation%90 != 0 {
			return nil, errors.New("rotation must be a multiple of 90")
		}
		dict["R"] = pdf.Integer(c.Rotation)
	}

	if bc, err := colorenc.Encode(c.BorderColor); err != nil {
		return nil, err
	} else if bc != nil {
		dict["BC"] = bc
	}

	if bg, err := colorenc.Encode(c.BackgroundColor); err != nil {
		return nil, err
	} else if bg != nil {
		dict["BG"] = bg
	}

	if c.Caption != "" {
		dict["CA"] = pdf.TextString(c.Caption)
	}
	if c.RolloverCaption != "" {
		dict["RC"] = pdf.TextString(c.RolloverCaption)
	}
	if c.DownCaption != "" {
		dict["AC"] = pdf.TextString(c.DownCaption)
	}

	if c.Icon != nil {
		ref, err := e.Embed(c.Icon)
		if err != nil {
			return nil, err
		}
		dict["I"] = ref
	}
	if c.RolloverIcon != nil {
		ref, err := e.Embed(c.RolloverIcon)
		if err != nil {
			return nil, err
		}
		dict["RI"] = ref
	}
	if c.DownIcon != nil {
		ref, err := e.Embed(c.DownIcon)
		if err != nil {
			return nil, err
		}
		dict["IX"] = ref
	}

	if c.IconFit != nil {
		ref, err := e.Embed(c.IconFit)
		if err != nil {
			return nil, err
		}
		dict["IF"] = ref
	}

	if c.TextPosition != 0 {
		if c.TextPosition < 0 || c.TextPosition > 6 {
			return nil, errors.New("invalid text position")
		}
		dict["TP"] = pdf.Integer(c.TextPosition)
	}

	if c.SingleUse {
		return dict, nil
	}

	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
