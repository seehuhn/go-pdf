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

type Flags uint16

const (
	// FlagInvisible applies only to annotations which do not belong to one of the standard annotation types and for which no annotation handler is available
	FlagInvisible Flags = 1 << 0

	// FlagHidden (PDF 1.2) do not render the annotation or allow it to interact with the user, regardless of its annotation type or whether an annotation handler is available
	FlagHidden Flags = 1 << 1

	// FlagPrint (PDF 1.2) print the annotation when the page is printed unless the Hidden flag is also set
	FlagPrint Flags = 1 << 2

	// FlagNoZoom (PDF 1.3) do not scale the annotation's appearance to match the magnification of the page
	FlagNoZoom Flags = 1 << 3

	// FlagNoRotate (PDF 1.3) do not rotate the annotation's appearance to match the rotation of the page
	FlagNoRotate Flags = 1 << 4

	// FlagNoView (PDF 1.3) do not render the annotation on the screen or allow it to interact with the user
	FlagNoView Flags = 1 << 5

	// FlagReadOnly (PDF 1.3) do not allow the annotation to interact with the user
	FlagReadOnly Flags = 1 << 6

	// FlagLocked (PDF 1.4) do not allow the annotation to be deleted or its properties to be modified by the user
	FlagLocked Flags = 1 << 7

	// FlagToggleNoView (PDF 1.5) invert the interpretation of the NoView flag for annotation selection and mouse hovering
	FlagToggleNoView Flags = 1 << 8

	// FlagLockedContents (PDF 1.7) do not allow the contents of the annotation to be modified by the user
	FlagLockedContents Flags = 1 << 9
)
