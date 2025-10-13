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

// PDF 2.0 sections: 12.6.2 12.6.4.10

package action

import (
	"seehuhn.de/go/pdf"
)

// Movie represents a movie action that plays a movie.
//
// Deprecated in PDF 2.0.
type Movie struct {
	// Annotation is the movie annotation to play.
	Annotation pdf.Object

	// T is the title of the movie annotation.
	T pdf.String

	// Operation specifies the operation to perform.
	Operation pdf.Name

	// Next is the sequence of actions to perform after this action.
	Next ActionList
}

// ActionType returns "Movie".
// This implements the [Action] interface.
func (a *Movie) ActionType() Type { return TypeMovie }

func (a *Movie) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "Movie action", pdf.V1_2); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name(TypeMovie),
	}

	if a.Annotation != nil {
		dict["Annotation"] = a.Annotation
	}

	if len(a.T) > 0 {
		dict["T"] = a.T
	}

	if a.Operation != "" {
		dict["Operation"] = a.Operation
	}

	if next, err := a.Next.Encode(rm); err != nil {
		return nil, err
	} else if next != nil {
		dict["Next"] = next
	}

	return dict, nil
}

func decodeMovie(x *pdf.Extractor, dict pdf.Dict) (*Movie, error) {
	t, _ := x.GetString(dict["T"])
	operation, _ := x.GetName(dict["Operation"])

	next, err := DecodeActionList(x, dict["Next"])
	if err != nil {
		return nil, err
	}

	return &Movie{
		Annotation: dict["Annotation"],
		T:          t,
		Operation:  operation,
		Next:       next,
	}, nil
}
