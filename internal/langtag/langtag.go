// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
