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

// Package langtag reads the BCP 47 language tags found in PDF files.
package langtag

import (
	"errors"

	"golang.org/x/text/language"
)

// Parse returns the language tag given by s, or [language.Und] if s does not
// identify a language.
//
// A malformed tag gives no language.  A well-formed tag may contain subtags
// which are not in the IANA registry; these are dropped, leaving a more
// general tag which is still true.  This does not apply to the primary
// language subtag: once it is gone, the region or script left behind would
// let a consumer infer a language the file never gave (und-DE suggests
// German), so the whole tag is dropped instead.
func Parse(s string) language.Tag {
	tag, err := language.Parse(s)
	if err == nil {
		return tag
	}
	if _, unknownSubtag := errors.AsType[language.ValueError](err); !unknownSubtag {
		return language.Und
	}
	if base, _, _ := tag.Raw(); base.String() == "und" {
		return language.Und
	}
	return tag
}
