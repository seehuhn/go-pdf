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

package acroform

import (
	"bytes"
	"math"

	"seehuhn.de/go/pdf"
)

// Field attributes (FT, Ff, V, DV, DA, Q, MaxLen) are inheritable in a PDF file
// (12.7.4.1, 12.7.4.3): a field without its own value takes its nearest
// ancestor's. The decoder flattens this away, so every terminal field carries
// its effective values. The encoder restores it: where moving a value from a
// set of children up into their common parent shrinks the file, it does so.
//
// The cost model mirrors the one used for inherited page attributes (see the
// pagetree package): a value is hoisted only when the resulting per-entry byte
// savings are non-negative. The two key facts that shape it:
//
//   - A child cannot override an inherited value with "absent" — omitting an
//     entry means "inherit" (an explicit null is equivalent to omission, 7.3.9).
//     So an attribute may be hoisted only when every child has an explicit value
//     for it (hoistUnanimous), unless the attribute has a writable default that
//     a child can use to opt back out (hoistWithDefault).
//   - V, DV and Opt are never hoisted: V/DV are rarely shared and their format
//     depends on the field type, and Opt is positionally tied to one field's
//     widget list.

// formatObj returns the textual PDF representation of obj, used to compare and
// size candidate values when factoring.
func formatObj(obj pdf.Object) string {
	var buf bytes.Buffer
	if err := pdf.Format(&buf, 0, obj); err != nil {
		// Format does not fail writing to a bytes.Buffer; if the impossible
		// happens, fall back to a unique-enough marker that blocks hoisting.
		return "\x00"
	}
	return buf.String()
}

// hoistUnanimous moves an inheritable attribute up from children into parent
// when every child carries it and doing so does not grow the file. It is used
// for attributes (FT, DA, MaxLen) that have no writable default: a child cannot
// re-express "no value", so the value may move up only when all children share
// the key. The most common value is chosen; matching children drop their own
// entry and inherit it from parent.
func hoistUnanimous(key pdf.Name, parent pdf.Dict, children []pdf.Dict) {
	n := len(children)
	if n == 0 {
		return
	}
	repr := make([]string, n)
	count := make(map[string]int)
	for i, child := range children {
		val, ok := child[key]
		if !ok {
			// a missing key cannot be expressed under inheritance
			return
		}
		r := formatObj(val)
		repr[i] = r
		count[r]++
	}

	// pick the value minimizing the byte cost, breaking ties on the
	// representation so the result does not depend on map iteration order
	bestDiff := math.MaxInt
	var bestRepr string
	l := len(key) + 3 // "/" + key + " " + value + "\n"
	for r, k := range count {
		diff := (l + len(r)) * (1 - k)
		if diff < bestDiff || (diff == bestDiff && r < bestRepr) {
			bestDiff = diff
			bestRepr = r
		}
	}
	// hoist only when it actually saves bytes (the value is shared)
	if bestDiff >= 0 {
		return
	}

	// the matching children all carry the same value; hoist it into parent and
	// drop it from each, so they inherit it instead
	for i, child := range children {
		if repr[i] == bestRepr {
			parent[key] = child[key]
			delete(child, key)
		}
	}
}

// hoistWithDefault moves an inheritable attribute with a writable default value
// (Ff and Q both default to 0) up from children into parent. A child can opt out
// of an inherited value by writing the default explicitly, so this also strips
// redundant explicit defaults and, when a non-default value is hoisted, sets the
// default explicitly on the children that need it.
//
// parent always ends up with a definite value (the hoisted value, or the
// default when some child relies on it), so that a parent dictionary reused as a
// child one level up never reads as "absent" by mistake.
func hoistWithDefault(key pdf.Name, def pdf.Object, parent pdf.Dict, children []pdf.Dict) {
	n := len(children)
	if n == 0 {
		return
	}
	defaultString := formatObj(def)

	repr := make([]string, n)
	count := make(map[string]int)
	numDefault := 0
	for i, child := range children {
		val, ok := child[key]
		if !ok {
			repr[i] = defaultString
			numDefault++
			continue
		}
		r := formatObj(val)
		repr[i] = r
		count[r]++
		if r == defaultString {
			// a redundant explicit default; the child can inherit it instead
			numDefault++
			delete(child, key)
		}
	}

	bestDiff := 0
	bestRepr := defaultString
	l := len(key) + 3
	for r, k := range count {
		if r == defaultString {
			continue
		}
		diff := (l+len(r))*(1-k) + numDefault*(l+len(defaultString))
		// a tie prefers moving the value up, enabling further hoisting; ties
		// between values are broken on the representation so the result is
		// independent of map order
		if diff < bestDiff || (diff == bestDiff && (bestRepr == defaultString || r < bestRepr)) {
			bestDiff = diff
			bestRepr = r
		}
	}

	if bestRepr == defaultString {
		if numDefault != 0 {
			parent[key] = def
		}
		return
	}

	// hoist the chosen value into parent and drop it from the matching children;
	// children that relied on the default now write it explicitly to override the
	// hoisted value
	for i, child := range children {
		switch repr[i] {
		case bestRepr:
			parent[key] = child[key]
			delete(child, key)
		case defaultString:
			child[key] = def
		}
	}
}
