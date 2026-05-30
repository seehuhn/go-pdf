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

package color

// Scratch-buffer slots on the shared [icc.Workspace] passed to Space.ToXYZ.
// Each slot is an independent reusable buffer. Three are needed because the
// deepest nested conversion chain — Indexed → Separation/DeviceN → ICCBased →
// Transform — keeps three caller-level buffers live at once, and each is passed
// as the input to a nested conversion whose own output uses a different slot, so
// the three must not alias. (Separation/DeviceN alternates may not themselves be
// Separation/DeviceN, so slotAlt never nests within itself; nested Indexed reuses
// slotIdx safely because the index is read before the slot is overwritten.)
const (
	slotNorm = iota // SpaceICCBased: device→[0,1] normalisation
	slotAlt         // Separation/DeviceN: tint-transform alternate output
	slotIdx         // Indexed: palette-lookup result
)
