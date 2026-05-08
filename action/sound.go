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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/sound"
)

// PDF 2.0 sections: 12.6.2 12.6.4.9

// Sound represents a sound action that plays a sound.
//
// Deprecated in PDF 2.0.
type Sound struct {
	// Sound (required) is the sound object to play.
	Sound *sound.Sound

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
// This implements the [pdf.Action] interface.
func (a *Sound) ActionType() pdf.Name  { return TypeSound }
func (a *Sound) GetNext() []pdf.Action { return []pdf.Action(a.Next) }

func (a *Sound) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Sound action", pdf.V1_2); err != nil {
		return nil, err
	}
	if a.Sound == nil {
		return nil, errors.New("Sound action must have a Sound object")
	}
	if a.Volume < -1 || a.Volume > 1 {
		return nil, errors.New("Sound action Volume must be in range -1.0 to 1.0")
	}

	soundObj, err := rm.Embed(a.Sound)
	if err != nil {
		return nil, err
	}
	dict := pdf.Dict{
		"S":     pdf.Name(TypeSound),
		"Sound": soundObj,
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Action")
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

func decodeSound(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*Sound, error) {
	soundObj, err := pdf.ExtractorGet(x, path, dict["Sound"], sound.Extract)
	if err != nil {
		return nil, err
	}

	volume := 1.0
	if dict["Volume"] != nil {
		v, err := pdf.Optional(x.GetNumber(path, dict["Volume"]))
		if err != nil {
			return nil, err
		}
		volume = v
	}

	synchronous, err := pdf.Optional(x.GetBoolean(path, dict["Synchronous"]))
	if err != nil {
		return nil, err
	}
	repeat, err := pdf.Optional(x.GetBoolean(path, dict["Repeat"]))
	if err != nil {
		return nil, err
	}
	mix, err := pdf.Optional(x.GetBoolean(path, dict["Mix"]))
	if err != nil {
		return nil, err
	}

	next, err := pdf.ExtractorGet(x, path, dict["Next"], DecodeActionList)
	if err != nil {
		return nil, err
	}

	return &Sound{
		Sound:       soundObj,
		Volume:      volume,
		Synchronous: bool(synchronous),
		Repeat:      bool(repeat),
		Mix:         bool(mix),
		Next:        next,
	}, nil
}
