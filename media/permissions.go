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
)

// TempFilePermission indicates the circumstances under which it is acceptable
// to write a temporary file in order to play a media clip.
//
// When encoding, an empty value is treated as [TempNever].
type TempFilePermission string

// Valid values for the [TempFilePermission] type.
const (
	TempNever   TempFilePermission = "TEMPNEVER"   // never allowed
	TempExtract TempFilePermission = "TEMPEXTRACT" // allowed if content extraction is permitted
	TempAccess  TempFilePermission = "TEMPACCESS"  // allowed if extraction for accessibility is permitted
	TempAlways  TempFilePermission = "TEMPALWAYS"  // always allowed
)

func (p TempFilePermission) isValid() bool {
	switch p {
	case TempNever, TempExtract, TempAccess, TempAlways:
		return true
	default:
		return false
	}
}

// MediaPermissions controls the use of media data.
type MediaPermissions struct {
	// TempFile indicates when it is acceptable to write a temporary file in
	// order to play a media clip.
	TempFile TempFilePermission

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractMediaPermissions reads a media permissions dictionary.
func ExtractMediaPermissions(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*MediaPermissions, error) {
	dict, err := x.GetDictTyped(path, obj, "MediaPermissions")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media permissions dictionary")
	}

	p := &MediaPermissions{SingleUse: isDirect}

	if tf, err := pdf.Optional(x.GetString(path, dict["TF"])); err != nil {
		return nil, err
	} else if v := TempFilePermission(tf); v.isValid() {
		p.TempFile = v
	}

	return p, nil
}

// Embed converts the media permissions to its PDF representation.
func (p *MediaPermissions) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media permissions", pdf.V1_5); err != nil {
		return nil, err
	}
	if p.TempFile != "" && !p.TempFile.isValid() {
		return nil, pdf.Error("media permissions: invalid TempFile value")
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaPermissions")
	}
	if p.TempFile != "" {
		dict["TF"] = pdf.String(p.TempFile)
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
