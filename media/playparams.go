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

// FitMode specifies how a visual media type that does not exactly fit its play
// rectangle shall be treated.  The zero value [FitUnspecified] means the
// author has no preference.
type FitMode int

// Valid values for the [FitMode] type.
const (
	FitUnspecified FitMode = iota // author has no preference
	FitMeet                       // scale preserving aspect ratio, show all content
	FitSlice                      // scale preserving aspect ratio, fill the rectangle
	FitFill                       // scale to fill, ignoring aspect ratio
	FitScroll                     // do not scale; provide a scrolling interface
	FitHidden                     // do not scale; clip to the play rectangle
)

// fitToPDF maps a FitMode to its PDF integer value, returning ok=false when
// the mode is unspecified (the F entry should then be omitted).
func (f FitMode) fitToPDF() (pdf.Integer, bool) {
	switch f {
	case FitMeet:
		return 0, true
	case FitSlice:
		return 1, true
	case FitFill:
		return 2, true
	case FitScroll:
		return 3, true
	case FitHidden:
		return 4, true
	default:
		return 0, false
	}
}

// fitFromPDF maps a PDF integer value to a FitMode, returning FitUnspecified
// for the player-default value 5 and any unrecognised value.
func fitFromPDF(v pdf.Integer) FitMode {
	switch v {
	case 0:
		return FitMeet
	case 1:
		return FitSlice
	case 2:
		return FitFill
	case 3:
		return FitScroll
	case 4:
		return FitHidden
	default:
		return FitUnspecified
	}
}

// MediaPlayParameters specifies how a media object should be played.
type MediaPlayParameters struct {
	// Players (optional) identifies players that are valid and not valid for
	// playing the media.
	Players *MediaPlayers

	// MustHonour holds the parameters that must be honoured for the media
	// play parameters to be viable.
	MustHonour *MediaPlayEntries

	// BestEffort holds the parameters that should be honoured on a best-effort
	// basis.
	BestEffort *MediaPlayEntries

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// MediaPlayEntries holds the entries of a media play parameters MH or BE
// dictionary.
type MediaPlayEntries struct {
	// Volume, if set, is the desired volume as a percentage of the recorded
	// level.  Zero is equivalent to mute.
	Volume optional.UInt

	// Controller, if set, controls whether a player controller user interface
	// is displayed.
	Controller optional.Bool

	// Fit specifies how media that does not fit the play rectangle is treated.
	Fit FitMode

	// Duration (optional) is the play duration.
	Duration *MediaDuration

	// AutoPlay, if set, controls whether the media plays automatically when
	// activated.
	AutoPlay optional.Bool

	// RepeatCount, if set, is the number of iterations of the duration to
	// repeat.  Zero means repeat forever.
	RepeatCount optional.Float64
}

// isEmpty reports whether no entry is set.
func (p *MediaPlayEntries) isEmpty() bool {
	if p == nil {
		return true
	}
	_, volSet := p.Volume.Get()
	_, ctrlSet := p.Controller.Get()
	_, autoSet := p.AutoPlay.Get()
	_, rcSet := p.RepeatCount.Get()
	return !volSet && !ctrlSet && p.Fit == FitUnspecified &&
		p.Duration == nil && !autoSet && !rcSet
}

// ExtractMediaPlayParameters reads a media play parameters dictionary.
func ExtractMediaPlayParameters(c pdf.Cursor, obj pdf.Object, isDirect bool) (*MediaPlayParameters, error) {
	dict, err := c.DictTyped(obj, "MediaPlayParams")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media play parameters dictionary")
	}

	p := &MediaPlayParameters{SingleUse: isDirect}

	if pl, err := pdf.DecodeOptional(c, dict["PL"], ExtractMediaPlayers); err != nil {
		return nil, err
	} else {
		p.Players = pl
	}

	if p.MustHonour, err = extractPlayEntries(c, dict["MH"]); err != nil {
		return nil, err
	}
	if p.BestEffort, err = extractPlayEntries(c, dict["BE"]); err != nil {
		return nil, err
	}

	return p, nil
}

func extractPlayEntries(c pdf.Cursor, obj pdf.Object) (*MediaPlayEntries, error) {
	dict, err := pdf.Optional(c.Dict(obj))
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	p := &MediaPlayEntries{}

	if dict["V"] != nil {
		if v, err := pdf.Optional(c.Integer(dict["V"])); err != nil {
			return nil, err
		} else if v >= 0 {
			p.Volume.Set(uint(v))
		}
	}
	if dict["C"] != nil {
		if c, err := pdf.Optional(c.Boolean(dict["C"])); err != nil {
			return nil, err
		} else {
			p.Controller.Set(bool(c))
		}
	}
	if dict["F"] != nil {
		if f, err := pdf.Optional(c.Integer(dict["F"])); err != nil {
			return nil, err
		} else {
			p.Fit = fitFromPDF(f)
		}
	}
	if d, err := pdf.DecodeOptional(c, dict["D"], ExtractMediaDuration); err != nil {
		return nil, err
	} else {
		p.Duration = d
	}
	if dict["A"] != nil {
		if a, err := pdf.Optional(c.Boolean(dict["A"])); err != nil {
			return nil, err
		} else {
			p.AutoPlay.Set(bool(a))
		}
	}
	if dict["RC"] != nil {
		if rc, err := pdf.Optional(c.Number(dict["RC"])); err != nil {
			return nil, err
		} else if rc >= 0 {
			p.RepeatCount.Set(rc)
		}
	}

	if p.isEmpty() {
		return nil, nil
	}
	return p, nil
}

// Embed converts the media play parameters to its PDF representation.
func (p *MediaPlayParameters) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media play parameters", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaPlayParams")
	}

	if p.Players != nil {
		pl, err := e.Embed(p.Players)
		if err != nil {
			return nil, err
		}
		dict["PL"] = pl
	}

	if !p.MustHonour.isEmpty() {
		mh, err := p.MustHonour.toDict(e)
		if err != nil {
			return nil, err
		}
		dict["MH"] = mh
	}
	if !p.BestEffort.isEmpty() {
		be, err := p.BestEffort.toDict(e)
		if err != nil {
			return nil, err
		}
		dict["BE"] = be
	}

	if p.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

func (p *MediaPlayEntries) toDict(e *pdf.EmbedHelper) (pdf.Dict, error) {
	dict := pdf.Dict{}

	if v, ok := p.Volume.Get(); ok {
		dict["V"] = pdf.Integer(v)
	}
	if c, ok := p.Controller.Get(); ok {
		dict["C"] = pdf.Boolean(c)
	}
	if v, ok := p.Fit.fitToPDF(); ok {
		dict["F"] = v
	}
	if p.Duration != nil {
		d, err := e.Embed(p.Duration)
		if err != nil {
			return nil, err
		}
		dict["D"] = d
	}
	if a, ok := p.AutoPlay.Get(); ok {
		dict["A"] = pdf.Boolean(a)
	}
	if rc, ok := p.RepeatCount.Get(); ok {
		if rc < 0 {
			return nil, pdf.Error("media play parameters: RepeatCount must not be negative")
		}
		dict["RC"] = pdf.Number(rc)
	}

	return dict, nil
}

// DurationKind selects how the duration of a [MediaDuration] is specified.
type DurationKind pdf.Name

// Valid values for the [DurationKind] type.
const (
	DurationIntrinsic DurationKind = "I" // the intrinsic duration of the media
	DurationInfinity  DurationKind = "F" // infinity
	DurationExplicit  DurationKind = "T" // the duration given by the Time entry
)

// MediaDuration specifies a play duration.
type MediaDuration struct {
	// Kind selects how the duration is specified.
	Kind DurationKind

	// Time is the explicit duration.  It is used only when Kind is
	// [DurationExplicit].
	Time *Timespan

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractMediaDuration reads a media duration dictionary.
func ExtractMediaDuration(c pdf.Cursor, obj pdf.Object, isDirect bool) (*MediaDuration, error) {
	dict, err := c.DictTyped(obj, "MediaDuration")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media duration dictionary")
	}

	s, err := pdf.Optional(c.Name(dict["S"]))
	if err != nil {
		return nil, err
	}

	d := &MediaDuration{SingleUse: isDirect}
	switch DurationKind(s) {
	case DurationIntrinsic, DurationInfinity:
		d.Kind = DurationKind(s)
	case DurationExplicit:
		t, err := pdf.Decode(c, dict["T"], ExtractTimespan)
		if err != nil {
			return nil, err
		} else if t == nil {
			return nil, pdf.Error("media duration missing T entry")
		}
		d.Kind = DurationExplicit
		d.Time = t
	default:
		return nil, pdf.Error("unknown media duration subtype: " + string(s))
	}

	return d, nil
}

// Embed converts the media duration to its PDF representation.
func (d *MediaDuration) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media duration", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaDuration")
	}

	switch d.Kind {
	case DurationIntrinsic, DurationInfinity:
		dict["S"] = pdf.Name(d.Kind)
	case DurationExplicit:
		if d.Time == nil {
			return nil, pdf.Error("media duration: Time is required when Kind is explicit")
		}
		t, err := e.Embed(d.Time)
		if err != nil {
			return nil, err
		}
		dict["S"] = pdf.Name("T")
		dict["T"] = t
	default:
		return nil, pdf.Error("media duration: invalid Kind")
	}

	if d.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
