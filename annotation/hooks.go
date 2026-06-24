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

package annotation

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/formhooks"
)

// register the widget plumbing used by the acroform package, so that it need
// not be part of the public API
func init() {
	formhooks.EncodeWidgetEntries = func(rm *pdf.ResourceManager, widget any) (pdf.Dict, error) {
		w, ok := widget.(*Widget)
		if !ok {
			return nil, errors.New("not a widget annotation")
		}
		return w.encodeOwnEntries(rm)
	}
}
