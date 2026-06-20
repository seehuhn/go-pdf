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

package movie

import (
	"encoding/binary"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 13.4

// Activation describes how a movie shall be played when its annotation
// is activated.  This represents a PDF movie activation dictionary.
//
// Deprecated in PDF 2.0.
type Activation struct {
	// Start is the starting time within the movie.
	// The zero value indicates that playback starts at the beginning of the
	// movie.
	Start Timestamp

	// Duration is the length of the segment to be played.
	// The zero value indicates that playback continues to the end of the
	// movie.
	Duration Timestamp

	// Rate is the initial speed at which to play the movie, 1.0 means normal
	// speed, 2.0 means twice normal speed, and so on.  Negative values play
	// the movie backward with respect to Start and Duration.
	//
	// On write, 0 can be used as a shorthand for 1.
	Rate float64

	// Volume is the initial sound volume in the range -1.0 to 1.0. The
	// absolute value of the volume is the fraction of the original sound
	// volume, and the sign determines whether to play the sound at all
	// (positive) or mute it (negative).
	Volume float64

	// ShowControls specifies whether to display a movie controller bar while
	// playing.
	ShowControls bool

	// Mode is the play mode for the movie.  An empty Mode is treated
	// as [ModeOnce] on encode.
	Mode Mode

	// Synchronous, when true, requires the movie player to
	// retain control until the movie is completed or dismissed.
	Synchronous bool

	// FWScale specifies the magnification factor at which the movie shall be
	// played in a floating window.  The zero value indicates that the movie
	// shall be played in the annotation rectangle (no floating window).
	FWScale Scale

	// FWPosition (optional) specifies the relative position of the
	// floating window on the screen, with both components in [0, 1].
	// nil means use the PDF default [0.5, 0.5] (centered).  Only
	// meaningful when FWScale is non-zero.
	FWPosition *Position

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// Timestamp is a point in time within a movie, expressed as a
// non-negative integer count of units.
type Timestamp struct {
	// Value is the time count in TimeScale units.  Must be
	// non-negative.
	Value int64

	// TimeScale is the number of units per second.  When zero, the
	// movie's intrinsic time scale is used.
	TimeScale int
}

// Scale is a rational magnification factor [Numerator / Denominator]
// for displaying a movie in a floating window.  The zero value
// indicates that the movie should play in the annotation rectangle
// rather than in a floating window.
type Scale struct {
	Numerator, Denominator int
}

// Position is a relative position on the screen for a floating movie
// window, with both components in the range [0, 1].
type Position struct {
	Horizontal, Vertical float64
}

// Mode is the play mode for a movie.
//
// A Mode written to a PDF file must be one of [ModeOnce],
// [ModeOpen], [ModeRepeat], or [ModePalindrome].
type Mode pdf.Name

// Standard play modes.
const (
	// ModeOnce plays the movie once and stops.
	ModeOnce Mode = "Once"

	// ModeOpen plays the movie and leaves the movie controller bar open.
	ModeOpen Mode = "Open"

	// ModeRepeat plays the movie repeatedly from beginning to end until
	// stopped.
	ModeRepeat Mode = "Repeat"

	// ModePalindrome plays the movie continuously forward and backward until
	// stopped.
	ModePalindrome Mode = "Palindrome"
)

// DefaultActivation is a sentinel value used as the Activation field
// of a movie annotation to request playback with the default
// parameters.
//
// This value must not be modified: it is shared global state, and
// recognition by [annotation.Movie] relies on pointer identity, so
// mutating any field would silently break every annotation that uses
// the sentinel.  Callers that need a customised activation must
// construct a fresh [*Activation] instead of copying or editing this
// one.
var DefaultActivation = &Activation{
	Rate:   1.0,
	Volume: 1.0,
	Mode:   ModeOnce,
}

// ExtractActivation reads a movie activation dictionary from the PDF
// file.  The isDirect parameter indicates whether the object was
// stored directly (true) or reached via an indirect reference (false).
func ExtractActivation(c pdf.Cursor, obj pdf.Object, isDirect bool) (*Activation, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing movie activation dictionary")
	}

	a := &Activation{
		Rate:      1.0,
		Volume:    1.0,
		Mode:      ModeOnce,
		SingleUse: isDirect,
	}

	a.Start = DecodeTimestamp(c, dict["Start"])
	a.Duration = DecodeTimestamp(c, dict["Duration"])

	if v := dict["Rate"]; v != nil {
		if num, err := pdf.Optional(c.Number(v)); err != nil {
			return nil, err
		} else if num != 0 {
			a.Rate = num
		}
		// Rate=0 collapses to the default 1.0
	}

	if v := dict["Volume"]; v != nil {
		if num, err := pdf.Optional(c.Number(v)); err != nil {
			return nil, err
		} else if num >= -1 && num <= 1 {
			a.Volume = num
		}
		// out-of-range values: silently keep default 1.0
	}

	if sc, err := pdf.Optional(c.Boolean(dict["ShowControls"])); err != nil {
		return nil, err
	} else {
		a.ShowControls = bool(sc)
	}

	if mode, err := pdf.Optional(c.Name(dict["Mode"])); err != nil {
		return nil, err
	} else if mode != "" {
		a.Mode = Mode(mode)
	}

	if sync, err := pdf.Optional(c.Boolean(dict["Synchronous"])); err != nil {
		return nil, err
	} else {
		a.Synchronous = bool(sync)
	}

	if arr, err := pdf.Optional(c.Array(dict["FWScale"])); err != nil {
		return nil, err
	} else if len(arr) >= 2 {
		n, errN := pdf.Optional(c.Integer(arr[0]))
		d, errD := pdf.Optional(c.Integer(arr[1]))
		if errN == nil && errD == nil && n > 0 && d > 0 {
			a.FWScale = Scale{Numerator: int(n), Denominator: int(d)}
		}
	}

	if arr, err := pdf.Optional(c.Array(dict["FWPosition"])); err != nil {
		return nil, err
	} else if len(arr) >= 2 {
		h, errH := pdf.Optional(c.Number(arr[0]))
		v, errV := pdf.Optional(c.Number(arr[1]))
		if errH == nil && errV == nil && h >= 0 && h <= 1 && v >= 0 && v <= 1 {
			a.FWPosition = &Position{Horizontal: h, Vertical: v}
		}
	}

	return a, nil
}

// Embed converts the movie activation dictionary to its PDF
// representation.
func (a *Activation) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "movie activation dictionary", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.Start.Value < 0 {
		return nil, fmt.Errorf("movie: Start.Value=%d must be non-negative", a.Start.Value)
	}
	if a.Start.TimeScale < 0 {
		return nil, fmt.Errorf("movie: Start.TimeScale=%d must be non-negative", a.Start.TimeScale)
	}
	if a.Duration.Value < 0 {
		return nil, fmt.Errorf("movie: Duration.Value=%d must be non-negative", a.Duration.Value)
	}
	if a.Duration.TimeScale < 0 {
		return nil, fmt.Errorf("movie: Duration.TimeScale=%d must be non-negative", a.Duration.TimeScale)
	}
	if a.Volume < -1 || a.Volume > 1 {
		return nil, fmt.Errorf("movie: Volume=%g out of range [-1, 1]", a.Volume)
	}
	if math.IsNaN(a.Rate) || math.IsInf(a.Rate, 0) {
		return nil, fmt.Errorf("movie: Rate=%g is not finite", a.Rate)
	}
	if a.FWScale != (Scale{}) {
		if a.FWScale.Numerator <= 0 || a.FWScale.Denominator <= 0 {
			return nil, fmt.Errorf("movie: FWScale [%d %d] must have positive components",
				a.FWScale.Numerator, a.FWScale.Denominator)
		}
	}
	if a.FWPosition != nil {
		if a.FWPosition.Horizontal < 0 || a.FWPosition.Horizontal > 1 ||
			a.FWPosition.Vertical < 0 || a.FWPosition.Vertical > 1 {
			return nil, fmt.Errorf("movie: FWPosition [%g %g] components must lie in [0, 1]",
				a.FWPosition.Horizontal, a.FWPosition.Vertical)
		}
	}
	dict := pdf.Dict{}

	if obj := EncodeTimestamp(a.Start); obj != nil {
		dict["Start"] = obj
	}
	if obj := EncodeTimestamp(a.Duration); obj != nil {
		dict["Duration"] = obj
	}
	if a.Rate != 0 && a.Rate != 1.0 {
		dict["Rate"] = pdf.Number(a.Rate)
	}
	if a.Volume != 1.0 {
		dict["Volume"] = pdf.Number(a.Volume)
	}
	if a.ShowControls {
		dict["ShowControls"] = pdf.Boolean(true)
	}
	if a.Mode != "" && a.Mode != ModeOnce {
		dict["Mode"] = pdf.Name(a.Mode)
	}
	if a.Synchronous {
		dict["Synchronous"] = pdf.Boolean(true)
	}
	if a.FWScale != (Scale{}) {
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

	if a.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// EncodeTimestamp returns the PDF representation of t, or nil if t is
// the zero value (meaning the entry should be omitted).
func EncodeTimestamp(t Timestamp) pdf.Object {
	if t == (Timestamp{}) {
		return nil
	}
	timeObj := encodeTime(t.Value)
	if t.TimeScale == 0 {
		return timeObj
	}
	return pdf.Array{timeObj, pdf.Integer(t.TimeScale)}
}

// encodeTime encodes a non-negative time count as either a PDF
// integer (when it fits in 32-bit signed) or an 8-byte
// two's-complement byte string (most significant byte first).
func encodeTime(v int64) pdf.Object {
	if v >= 0 && v <= math.MaxInt32 {
		return pdf.Integer(v)
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(v))
	return pdf.String(buf[:])
}

// DecodeTimestamp parses a movie-activation Start or Duration entry.
// Malformed entries are silently treated as omitted (zero value).
func DecodeTimestamp(c pdf.Cursor, obj pdf.Object) Timestamp {
	if obj == nil {
		return Timestamp{}
	}
	resolved, err := c.Resolve(obj)
	if err != nil {
		return Timestamp{}
	}
	switch v := resolved.(type) {
	case pdf.Integer:
		if v < 0 {
			return Timestamp{}
		}
		return Timestamp{Value: int64(v)}
	case pdf.String:
		if t, ok := decodeTime8(v); ok {
			return Timestamp{Value: t}
		}
		return Timestamp{}
	case pdf.Array:
		if len(v) < 2 {
			return Timestamp{}
		}
		t := decodeTimeOnly(c, v[0])
		scale, _ := pdf.Optional(c.Integer(v[1]))
		if scale <= 0 {
			return Timestamp{}
		}
		return Timestamp{Value: t, TimeScale: int(scale)}
	}
	return Timestamp{}
}

// decodeTimeOnly parses the time component of a [time scale] array,
// accepting either an integer or an 8-byte two's-complement string.
func decodeTimeOnly(c pdf.Cursor, obj pdf.Object) int64 {
	resolved, err := c.Resolve(obj)
	if err != nil {
		return 0
	}
	switch v := resolved.(type) {
	case pdf.Integer:
		if v < 0 {
			return 0
		}
		return int64(v)
	case pdf.String:
		if t, ok := decodeTime8(v); ok {
			return t
		}
	}
	return 0
}

// decodeTime8 parses an 8-byte big-endian two's-complement byte string
// as a non-negative int64.  Lengths other than 8 or negative results
// are rejected.
func decodeTime8(b pdf.String) (int64, bool) {
	if len(b) != 8 {
		return 0, false
	}
	v := int64(binary.BigEndian.Uint64([]byte(b)))
	if v < 0 {
		return 0, false
	}
	return v, true
}
