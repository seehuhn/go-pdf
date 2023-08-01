// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package cmap

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript"
)

func ExtractRaw(r pdf.Getter, ref pdf.Object) (postscript.Dict, error) {
	stream, err := pdf.GetStream(r, ref)
	if err != nil {
		return nil, err
	}
	cmapBody, err := pdf.DecodeStream(r, stream, 0)
	if err != nil {
		return nil, err
	}
	return ReadRaw(cmapBody)
}

func ReadRaw(r io.Reader) (postscript.Dict, error) {
	intp := postscript.NewInterpreter()
	intp.MaxOps = 1_000_000 // TODO(voss): measure what is required
	err := intp.Execute(r)
	if err != nil {
		return nil, err
	}

	var cmap postscript.Dict
	for name, val := range intp.CMapDirectory {
		var ok bool
		cmap, ok = val.(postscript.Dict)
		if !ok {
			continue
		}
		if _, ok := cmap["CMapName"].(postscript.Name); !ok {
			cmap["CMapName"] = postscript.Name(name)
		}
	}
	if cmap == nil {
		return nil, fmt.Errorf("no valid CMap found")
	}

	return cmap, nil
}
