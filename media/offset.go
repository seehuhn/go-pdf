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
	"errors"

	"seehuhn.de/go/pdf"
)

// Timespan specifies a length of time, in seconds.
type Timespan struct {
	// Seconds is the number of seconds in the timespan.  It must not be
	// negative.
	Seconds float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractTimespan reads a timespan dictionary.
func ExtractTimespan(c pdf.Cursor, obj pdf.Object, isDirect bool) (*Timespan, error) {
	dict, err := c.DictTyped(obj, "Timespan")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing timespan dictionary")
	}

	v, err := pdf.Optional(c.Number(dict["V"]))
	if err != nil {
		return nil, err
	} else if v < 0 {
		return nil, pdf.Error("invalid timespan value")
	}

	return &Timespan{Seconds: v, SingleUse: isDirect}, nil
}

// Embed converts the timespan to its PDF representation.
func (t *Timespan) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "timespan", pdf.V1_5); err != nil {
		return nil, err
	}
	if t.Seconds < 0 {
		return nil, errors.New("timespan: Seconds must not be negative")
	}

	dict := pdf.Dict{
		"S": pdf.Name("S"),
		"V": pdf.Number(t.Seconds),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Timespan")
	}

	if t.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// MediaOffset specifies an offset into a media object, in terms of time,
// frames or markers.  It is implemented by [MediaOffsetTime],
// [MediaOffsetFrame] and [MediaOffsetMarker].
type MediaOffset interface {
	pdf.Embedder
	isMediaOffset()
}

// MediaOffsetTime specifies an offset as a temporal offset.
type MediaOffsetTime struct {
	Time *Timespan

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// MediaOffsetFrame specifies an offset as a frame number.
type MediaOffsetFrame struct {
	// Frame is the frame number.  Frame numbers begin at 0 and must not be
	// negative.
	Frame int

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// MediaOffsetMarker specifies an offset as a named marker.
type MediaOffsetMarker struct {
	// Marker identifies a named offset within a media object.
	Marker string

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

func (*MediaOffsetTime) isMediaOffset()   {}
func (*MediaOffsetFrame) isMediaOffset()  {}
func (*MediaOffsetMarker) isMediaOffset() {}

// ExtractMediaOffset reads a media offset dictionary and dispatches on its
// subtype.
func ExtractMediaOffset(c pdf.Cursor, obj pdf.Object, isDirect bool) (MediaOffset, error) {
	dict, err := c.DictTyped(obj, "MediaOffset")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media offset dictionary")
	}

	s, err := pdf.Optional(c.Name(dict["S"]))
	if err != nil {
		return nil, err
	}

	switch s {
	case "T":
		t, err := pdf.Decode(c, dict["T"], ExtractTimespan)
		if err != nil {
			return nil, err
		} else if t == nil {
			return nil, pdf.Error("media offset time missing T entry")
		}
		return &MediaOffsetTime{Time: t, SingleUse: isDirect}, nil
	case "F":
		f, err := pdf.Optional(c.Integer(dict["F"]))
		if err != nil {
			return nil, err
		} else if f < 0 {
			return nil, pdf.Error("invalid media offset frame")
		}
		return &MediaOffsetFrame{Frame: int(f), SingleUse: isDirect}, nil
	case "M":
		m, err := pdf.Optional(c.TextString(dict["M"]))
		if err != nil {
			return nil, err
		}
		return &MediaOffsetMarker{Marker: string(m), SingleUse: isDirect}, nil
	default:
		return nil, pdf.Error("unknown media offset subtype: " + string(s))
	}
}

func (o *MediaOffsetTime) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media offset", pdf.V1_5); err != nil {
		return nil, err
	}
	if o.Time == nil {
		return nil, errors.New("media offset time: Time is required")
	}
	t, err := e.Embed(o.Time)
	if err != nil {
		return nil, err
	}
	dict := pdf.Dict{
		"S": pdf.Name("T"),
		"T": t,
	}
	return offsetResult(e, dict, o.SingleUse)
}

func (o *MediaOffsetFrame) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media offset", pdf.V1_5); err != nil {
		return nil, err
	}
	if o.Frame < 0 {
		return nil, errors.New("media offset frame: Frame must not be negative")
	}
	dict := pdf.Dict{
		"S": pdf.Name("F"),
		"F": pdf.Integer(o.Frame),
	}
	return offsetResult(e, dict, o.SingleUse)
}

func (o *MediaOffsetMarker) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media offset", pdf.V1_5); err != nil {
		return nil, err
	}
	dict := pdf.Dict{
		"S": pdf.Name("M"),
		"M": pdf.TextString(o.Marker),
	}
	return offsetResult(e, dict, o.SingleUse)
}

func offsetResult(e *pdf.EmbedHelper, dict pdf.Dict, singleUse bool) (pdf.Native, error) {
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaOffset")
	}
	if singleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
