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

package annotation

// State represents a PDF annotation state.
type State int

// State represents the valid values of a [State] variable.
const (
	// StateUnknown indicates that no /State or /StateModel field are present.
	StateUnknown State = iota

	// Values following the "Marked" state model.
	StateUnmarked
	StateMarked

	// Values following the "Review" state model.
	StateAccepted
	StateRejected
	StateCancelled
	StateCompleted
	StateNone
)
