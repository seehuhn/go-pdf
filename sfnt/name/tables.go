// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package name

import (
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/text/language"
)

// Tables contains the information from a "name" table.
type Tables map[string]*Table

// Choose select the table which best matches the given language preferences.
func (tt Tables) Choose(prefs ...language.Tag) (*Table, language.Confidence) {
	keys := maps.Keys(tt)
	if len(keys) == 0 {
		return nil, language.No
	}

	pref := make(map[string]int)
	for key, t := range tt {
		numFilled := 10 * len(t.keys())
		if key == "en-US" {
			numFilled += 55
		} else if key == "en" || strings.HasPrefix(key, "en-") {
			numFilled += 5
		}
		pref[key] = numFilled
	}

	sort.Slice(keys, func(i, j int) bool {
		keyI := keys[i]
		prefI := pref[keyI]
		keyJ := keys[j]
		prefJ := pref[keyJ]
		if prefI != prefJ {
			return prefI > prefJ
		}
		return keyI < keyJ
	})

	tags := make([]language.Tag, len(keys))
	for i, key := range keys {
		tags[i] = language.MustParse(key)
	}
	matcher := language.NewMatcher(tags)

	_, index, confidence := matcher.Match(prefs...)
	return tt[keys[index]], confidence
}
