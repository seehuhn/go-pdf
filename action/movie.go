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

package action

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/movie"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 12.6.2 12.6.4.10

// Movie represents a movie action that plays a movie.
//
// A movie action dictionary carries the same playback parameters as a
// movie activation dictionary; values supplied here override those in
// the activation dictionary associated with the movie annotation.
//
// Every override field has a distinct "no override" zero value so that
// an absent wire entry is never conflated with an explicit override
// equal to the PDF default.  The encoding rules per field are listed on
// each field.
//
// Deprecated in PDF 2.0.
type Movie struct {
	// Annotation is an indirect reference to the movie annotation
	// to play.
	Annotation pdf.Reference

	// T is the title of the movie annotation.
	T pdf.String

	// Operation specifies the operation to perform.
	//
	// On write, an empty name can be used as a shorthand for [MovieOperationPlay].
	Operation MovieOperation

	// Start (optional override): nil means no override and the
	// annotation's activation value is used.  A non-nil value
	// overrides; a non-nil zero [movie.Timestamp] is valid and means
	// "start at the beginning".
	Start *movie.Timestamp

	// Duration (optional override): nil means no override.
	Duration *movie.Timestamp

	// Rate (optional override): the zero value means no override.
	// Any non-zero value (including the PDF default 1.0) is written
	// as an explicit override.
	Rate float64

	// Volume (optional override): the unset zero value means no
	// override.  When set, the value must lie in [-1.0, 1.0];
	// negative values mute the sound.
	Volume optional.Float64

	// ShowControls (optional override): the unset zero value means
	// no override.
	ShowControls optional.Bool

	// Mode (optional override): the empty string means no override.
	Mode movie.Mode

	// Synchronous (optional override): the unset zero value means
	// no override.
	Synchronous optional.Bool

	// FWScale (optional override): nil means no override.  A non-nil
	// value overrides; its Numerator and Denominator must both be
	// positive.
	FWScale *movie.Scale

	// FWPosition (optional override): nil means no override.  A
	// non-nil value overrides with both components in [0, 1].
	FWPosition *movie.Position

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Movie".
// This implements the [pdf.Action] interface.
func (a *Movie) ActionType() pdf.Name  { return TypeMovie }
func (a *Movie) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *Movie) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Movie action", pdf.V1_2); err != nil {
		return nil, err
	}

	if a.Start != nil {
		if a.Start.Value < 0 {
			return nil, fmt.Errorf("Movie action: Start.Value=%d must be non-negative", a.Start.Value)
		}
		if a.Start.TimeScale < 0 {
			return nil, fmt.Errorf("Movie action: Start.TimeScale=%d must be non-negative", a.Start.TimeScale)
		}
	}
	if a.Duration != nil {
		if a.Duration.Value < 0 {
			return nil, fmt.Errorf("Movie action: Duration.Value=%d must be non-negative", a.Duration.Value)
		}
		if a.Duration.TimeScale < 0 {
			return nil, fmt.Errorf("Movie action: Duration.TimeScale=%d must be non-negative", a.Duration.TimeScale)
		}
	}
	if math.IsNaN(a.Rate) || math.IsInf(a.Rate, 0) {
		return nil, fmt.Errorf("Movie action: Rate=%g is not finite", a.Rate)
	}
	if vol, ok := a.Volume.Get(); ok && (vol < -1 || vol > 1) {
		return nil, fmt.Errorf("Movie action: Volume=%g out of range [-1, 1]", vol)
	}
	if a.FWScale != nil {
		if a.FWScale.Numerator <= 0 || a.FWScale.Denominator <= 0 {
			return nil, fmt.Errorf("Movie action: FWScale [%d %d] must have positive components",
				a.FWScale.Numerator, a.FWScale.Denominator)
		}
	}
	if a.FWPosition != nil {
		if a.FWPosition.Horizontal < 0 || a.FWPosition.Horizontal > 1 ||
			a.FWPosition.Vertical < 0 || a.FWPosition.Vertical > 1 {
			return nil, fmt.Errorf("Movie action: FWPosition [%g %g] components must lie in [0, 1]",
				a.FWPosition.Horizontal, a.FWPosition.Vertical)
		}
	}
	dict := pdf.Dict{
		"S": pdf.Name(TypeMovie),
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
	}

	switch {
	case a.Annotation != 0 && len(a.T) > 0:
		return nil, errors.New("Movie action must not have both Annotation and T")
	case a.Annotation != 0:
		dict["Annotation"] = a.Annotation
	case len(a.T) > 0:
		dict["T"] = a.T
	default:
		return nil, errors.New("Movie action requires Annotation or T")
	}

	if a.Operation != "" && a.Operation != MovieOperationPlay {
		dict["Operation"] = pdf.Name(a.Operation)
	}

	if a.Start != nil {
		dict["Start"] = encodeTimestampOverride(*a.Start)
	}
	if a.Duration != nil {
		dict["Duration"] = encodeTimestampOverride(*a.Duration)
	}
	if a.Rate != 0 {
		dict["Rate"] = pdf.Number(a.Rate)
	}
	if vol, ok := a.Volume.Get(); ok {
		dict["Volume"] = pdf.Number(vol)
	}
	if sc, ok := a.ShowControls.Get(); ok {
		dict["ShowControls"] = pdf.Boolean(sc)
	}
	if a.Mode != "" {
		dict["Mode"] = pdf.Name(a.Mode)
	}
	if sync, ok := a.Synchronous.Get(); ok {
		dict["Synchronous"] = pdf.Boolean(sync)
	}
	if a.FWScale != nil {
		dict["FWScale"] = pdf.Array{
			pdf.Integer(a.FWScale.Numerator),
			pdf.Integer(a.FWScale.Denominator),
		}
	}
	if a.FWPosition != nil {
		dict["FWPosition"] = pdf.Array{
			pdf.Number(a.FWPosition.Horizontal),
			pdf.Number(a.FWPosition.Vertical),
		}
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

// encodeTimestampOverride encodes a Timestamp that is known to be a
// real override (callers handle the nil-pointer "no override" case).
// Unlike [movie.EncodeTimestamp], it never returns nil: a zero
// Timestamp{} encodes as integer 0 so the override remains visible on
// the wire.
func encodeTimestampOverride(t movie.Timestamp) pdf.Object {
	if t == (movie.Timestamp{}) {
		return pdf.Integer(0)
	}
	return movie.EncodeTimestamp(t)
}

func decodeMovie(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*Movie, error) {
	annotation, _ := dict["Annotation"].(pdf.Reference)
	t, err := pdf.Optional(x.GetString(path, dict["T"]))
	if err != nil {
		return nil, err
	}

	// the spec requires exactly one of Annotation or T
	switch {
	case annotation != 0 && len(t) > 0:
		t = nil
	case annotation == 0 && len(t) == 0:
		return nil, pdf.Error("Movie action missing both Annotation and T")
	}

	operation, err := pdf.Optional(x.GetName(path, dict["Operation"]))
	if err != nil {
		return nil, err
	}
	if operation == "" {
		operation = pdf.Name(MovieOperationPlay)
	}

	a := &Movie{
		Annotation: annotation,
		T:          t,
		Operation:  MovieOperation(operation),
	}

	if v := dict["Start"]; v != nil {
		ts := movie.DecodeTimestamp(x, path, v)
		a.Start = &ts
	}
	if v := dict["Duration"]; v != nil {
		ts := movie.DecodeTimestamp(x, path, v)
		a.Duration = &ts
	}

	if v := dict["Rate"]; v != nil {
		if num, err := pdf.Optional(x.GetNumber(path, v)); err != nil {
			return nil, err
		} else {
			a.Rate = num
		}
	}

	if v := dict["Volume"]; v != nil {
		if num, err := pdf.Optional(x.GetNumber(path, v)); err != nil {
			return nil, err
		} else if num >= -1 && num <= 1 {
			a.Volume.Set(num)
		}
		// out-of-range values: silently drop the override
	}

	if v := dict["ShowControls"]; v != nil {
		if sc, err := pdf.Optional(x.GetBoolean(path, v)); err != nil {
			return nil, err
		} else {
			a.ShowControls.Set(bool(sc))
		}
	}

	if mode, err := pdf.Optional(x.GetName(path, dict["Mode"])); err != nil {
		return nil, err
	} else if mode != "" {
		a.Mode = movie.Mode(mode)
	}

	if v := dict["Synchronous"]; v != nil {
		if sync, err := pdf.Optional(x.GetBoolean(path, v)); err != nil {
			return nil, err
		} else {
			a.Synchronous.Set(bool(sync))
		}
	}

	if arr, err := pdf.Optional(x.GetArray(path, dict["FWScale"])); err != nil {
		return nil, err
	} else if len(arr) >= 2 {
		n, errN := pdf.Optional(x.GetInteger(path, arr[0]))
		d, errD := pdf.Optional(x.GetInteger(path, arr[1]))
		if errN == nil && errD == nil && n > 0 && d > 0 {
			a.FWScale = &movie.Scale{Numerator: int(n), Denominator: int(d)}
		}
	}

	if arr, err := pdf.Optional(x.GetArray(path, dict["FWPosition"])); err != nil {
		return nil, err
	} else if len(arr) >= 2 {
		h, errH := pdf.Optional(x.GetNumber(path, arr[0]))
		v, errV := pdf.Optional(x.GetNumber(path, arr[1]))
		if errH == nil && errV == nil && h >= 0 && h <= 1 && v >= 0 && v <= 1 {
			a.FWPosition = &movie.Position{Horizontal: h, Vertical: v}
		}
	}

	next, err := pdf.ExtractorGet(x, path, dict["Next"], DecodeActionList)
	if err != nil {
		return nil, err
	}
	a.Next = next

	return a, nil
}

// MovieOperation specifies the operation to perform on a movie.
//
// A MovieOperation written to a PDF file must be one of
// [MovieOperationPlay], [MovieOperationStop], [MovieOperationPause],
// or [MovieOperationResume].
type MovieOperation pdf.Name

const (
	MovieOperationPlay   MovieOperation = "Play"
	MovieOperationStop   MovieOperation = "Stop"
	MovieOperationPause  MovieOperation = "Pause"
	MovieOperationResume MovieOperation = "Resume"
)
