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
	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.6.2 12.6.4.9

// Sound represents a sound action that plays a sound.
//
// Deprecated in PDF 2.0.
type Sound struct {
	// Sound is a reference to the sound object.
	Sound pdf.Reference

	// Volume is the volume at which to play the sound (-1.0 to 1.0).
	// Default is 1.0.
	Volume float64

	// Synchronous indicates whether to play synchronously.
	Synchronous bool

	// Repeat indicates whether to repeat the sound indefinitely.
	Repeat bool

	// Mix indicates whether to mix with other sounds.
	Mix bool

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Sound".
// This implements the [Action] interface.
func (a *Sound) ActionType() Type { return TypeSound }

func (a *Sound) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Sound action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.Sound == 0 {
		return nil, pdf.Error("Sound action must have a Sound reference")
	}

	dict := pdf.Dict{
		"S":     pdf.Name(TypeSound),
		"Sound": a.Sound,
	}

	if a.Volume != 1.0 {
		dict["Volume"] = pdf.Number(a.Volume)
	}
	if a.Synchronous {
		dict["Synchronous"] = pdf.Boolean(true)
	}
	if a.Repeat {
		dict["Repeat"] = pdf.Boolean(true)
	}
	if a.Mix {
		dict["Mix"] = pdf.Boolean(true)
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeSound(x *pdf.Extractor, dict pdf.Dict) (*Sound, error) {
	soundRef, ok := dict["Sound"].(pdf.Reference)
	if !ok {
		return nil, pdf.Error("Sound action missing or invalid Sound entry")
	}

	volume := 1.0
	if v, err := pdf.Optional(x.GetNumber(dict["Volume"])); err == nil {
		volume = v
	}

	synchronous, _ := pdf.Optional(x.GetBoolean(dict["Synchronous"]))
	repeat, _ := pdf.Optional(x.GetBoolean(dict["Repeat"]))
	mix, _ := pdf.Optional(x.GetBoolean(dict["Mix"]))

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Sound{
		Sound:       soundRef,
		Volume:      volume,
		Synchronous: bool(synchronous),
		Repeat:      bool(repeat),
		Mix:         bool(mix),
		Next:        next,
	}, nil
}
