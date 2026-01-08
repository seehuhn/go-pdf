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

package property

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 14.13.5

// AF represents an Associated Files property list.
// This is used with the AF marked-content tag to link associated files
// to sections of content in a content stream.
type AF struct {
	// MCID (optional) is the marked-content identifier for structure.
	MCID optional.Int

	// AssociatedFiles is an array of file specifications.
	// This corresponds to the MCAF entry in the property list (Table 409a).
	// Must contain at least one file specification.
	AssociatedFiles []*file.Specification

	// SingleUse controls whether the property list is embedded as a direct
	// object in the Properties resource dictionary (true) or as an indirect
	// object (false).
	SingleUse bool
}

var _ List = (*AF)(nil)

func (a *AF) Keys() []pdf.Name {
	var keys []pdf.Name
	keys = append(keys, "MCAF")
	if _, ok := a.MCID.Get(); ok {
		keys = append(keys, "MCID")
	}
	return keys
}

func (a *AF) Get(key pdf.Name) (*ResolvedObject, error) {
	switch key {
	case "MCID":
		if v, ok := a.MCID.Get(); ok {
			return &ResolvedObject{obj: v, x: nil}, nil
		}
	case "MCAF":
		w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(w)

		arr := make(pdf.Array, len(a.AssociatedFiles))
		for i, spec := range a.AssociatedFiles {
			embedded, err := rm.Embed(spec)
			if err != nil {
				return nil, err
			}
			arr[i] = embedded
		}

		err := rm.Close()
		if err != nil {
			return nil, err
		}

		x := pdf.NewExtractor(w)
		return &ResolvedObject{obj: arr, x: x}, nil
	}
	return nil, ErrNoKey
}

func (a *AF) IsDirect() bool {
	// AF can never be inline in content stream because it contains
	// file specifications with indirect references to embedded files.
	return false
}

func (a *AF) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if len(a.AssociatedFiles) == 0 {
		return nil, errors.New("AF property list requires at least one associated file")
	}

	dict := make(pdf.Dict)

	if mcid, ok := a.MCID.Get(); ok {
		dict["MCID"] = mcid
	}

	arr := make(pdf.Array, len(a.AssociatedFiles))
	for i, spec := range a.AssociatedFiles {
		embedded, err := rm.Embed(spec)
		if err != nil {
			return nil, err
		}
		arr[i] = embedded
	}
	dict["MCAF"] = arr

	if a.SingleUse {
		return dict, nil
	}

	ref := rm.AllocSelf()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
