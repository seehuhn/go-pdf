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

// Package appearance handles annotation appearance dictionaries.
//
// An annotation can have up to three appearances, each of them a form XObject:
// the normal appearance is shown while the annotation is not interacting with
// the user and is also the one which is printed, the rollover appearance while
// the pointer rests inside the annotation's active area, and the down
// appearance while the pointer button is held down there.  Each of the three
// can instead be given as one form per appearance state, in which case the
// annotation's AS entry selects the state to show.
//
// An appearance is drawn inside the annotation rectangle: the form's bounding
// box, transformed by the form matrix, is scaled and translated to fill the
// rectangle, so the appearance can be drawn in whichever coordinates suit it.
// [ToRect] computes this mapping.
package appearance

import (
	"errors"
	"maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
)

// PDF 2.0 sections: 12.5.5

// Dict represents an annotation appearance dictionary.
//
// A rollover or down appearance which is not set shows the normal appearance.
type Dict struct {
	// Normal is the annotation's normal appearance.
	// This is mutually exclusive with NormalMap.
	Normal *form.Form

	// NormalMap gives the annotation's normal appearance for each state.
	//
	// This is mutually exclusive with Normal.
	NormalMap map[pdf.Name]*form.Form

	// RollOver (optional) is the annotation's rollover appearance.
	//
	// This is mutually exclusive with RollOverMap.
	RollOver *form.Form

	// RollOverMap (optional) gives the annotation's rollover appearance for
	// each state.
	//
	// This is mutually exclusive with RollOver.
	RollOverMap map[pdf.Name]*form.Form

	// Down (optional) is the annotation's down appearance.
	//
	// This is mutually exclusive with DownMap.
	Down *form.Form

	// DownMap (optional) gives the annotation's down appearance for each
	// state.
	//
	// This is mutually exclusive with Down.
	DownMap map[pdf.Name]*form.Form

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder = (*Dict)(nil)

// ExtractDict reads an annotation appearance dictionary from the PDF object obj.
// If obj is absent or resolves to null, ExtractDict returns (nil, nil); callers
// can treat the missing entry as "no appearance".
func ExtractDict(c pdf.Cursor, obj pdf.Object, isDirect bool) (*Dict, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	res := &Dict{
		SingleUse: isDirect,
	}

	res.Normal, res.NormalMap, err = extractEntry(c, dict["N"])
	if err != nil {
		return nil, err
	}
	if res.Normal == nil && res.NormalMap == nil {
		return nil, pdf.Error("missing normal appearance")
	}

	// An absent or malformed /R or /D shows the normal appearance (§12.5.5).
	// The entry is filled in with the normal appearance itself, so that every
	// entry holds the appearance which is shown; Embed leaves the entry out
	// again, recognising it by the identity of the forms.
	rollOver, rollOverMap, err := extractEntry(c, dict["R"])
	if err != nil {
		return nil, err
	}
	if rollOver == nil && rollOverMap == nil {
		rollOver, rollOverMap = res.Normal, res.NormalMap
	}
	res.RollOver, res.RollOverMap = rollOver, rollOverMap

	down, downMap, err := extractEntry(c, dict["D"])
	if err != nil {
		return nil, err
	}
	if down == nil && downMap == nil {
		down, downMap = res.Normal, res.NormalMap
	}
	res.Down, res.DownMap = down, downMap

	return res, nil
}

// extractEntry reads one entry of an appearance dictionary, which holds either
// a single appearance stream or, for an annotation with appearance states, a
// stream per state.  An absent or malformed entry yields no appearance and no
// error; a state whose stream is malformed is left out of the map, and a map
// left with no state at all counts as absent in turn.
//
// Each stream is decoded from the object as it stands in the dictionary rather
// than from its resolved value, so that entries sharing a reference share one
// [form.Form].
func extractEntry(c pdf.Cursor, obj pdf.Object) (*form.Form, map[pdf.Name]*form.Form, error) {
	resolved, err := pdf.Optional(c.Resolve(obj))
	if err != nil {
		return nil, nil, err
	}

	switch resolved := resolved.(type) {
	case pdf.Dict:
		byState := make(map[pdf.Name]*form.Form)
		for state, stateObj := range resolved {
			if state == "" {
				continue
			}
			f, err := pdf.DecodeOptional(c, stateObj, extract.Form)
			if err != nil {
				return nil, nil, err
			}
			if f != nil {
				byState[state] = f
			}
		}
		if len(byState) == 0 {
			return nil, nil, nil
		}
		return nil, byState, nil

	case *pdf.Stream:
		f, err := pdf.DecodeOptional(c, obj, extract.Form)
		if err != nil {
			return nil, nil, err
		}
		if f == nil {
			return nil, nil, nil
		}
		return f, nil, nil
	}

	return nil, nil, nil
}

// Embed writes the appearance dictionary to the PDF file.
//
// This implements the [pdf.Embedder] interface.
func (d *Dict) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "appearance streams", pdf.V1_2); err != nil {
		return nil, err
	}

	if d.Normal != nil && len(d.NormalMap) > 0 {
		return nil, errors.New("Normal and NormalMap are mutually exclusive")
	}
	if d.Normal == nil && len(d.NormalMap) == 0 {
		return nil, errors.New("normal appearance is required")
	}
	if d.RollOver != nil && len(d.RollOverMap) > 0 {
		return nil, errors.New("RollOver and RollOverMap are mutually exclusive")
	}
	if d.Down != nil && len(d.DownMap) > 0 {
		return nil, errors.New("Down and DownMap are mutually exclusive")
	}

	dict := pdf.Dict{}

	normal, err := embedEntry(e, d.Normal, d.NormalMap)
	if err != nil {
		return nil, err
	}
	dict["N"] = normal

	// A rollover or down appearance which only repeats the normal appearance
	// is left out: a reader substitutes the normal appearance for the missing
	// entry (§12.5.5).
	if !d.repeatsNormal(d.RollOver, d.RollOverMap) {
		rollOver, err := embedEntry(e, d.RollOver, d.RollOverMap)
		if err != nil {
			return nil, err
		}
		dict["R"] = rollOver
	}

	if !d.repeatsNormal(d.Down, d.DownMap) {
		down, err := embedEntry(e, d.Down, d.DownMap)
		if err != nil {
			return nil, err
		}
		dict["D"] = down
	}

	if d.SingleUse {
		return dict, nil
	}

	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}

	return ref, nil
}

// Clone returns a shallow copy of the appearance dictionary.
//
// The appearance streams themselves are shared with the original rather than
// copied.  A form is one appearance stream in the file, and which entries hold
// the same one is what tells a rollover the file asks for from the normal
// appearance repeated; copying the forms would turn every repeat into an entry
// of its own.  Use [Dict.SetNormal] to replace one, which keeps the entries
// which repeat the normal appearance in step.
//
// The method is nil-safe: a nil dictionary yields nil.
func (d *Dict) Clone() *Dict {
	if d == nil {
		return nil
	}

	res := *d
	res.NormalMap = maps.Clone(d.NormalMap)
	res.RollOverMap = maps.Clone(d.RollOverMap)
	res.DownMap = maps.Clone(d.DownMap)
	return &res
}

// SetNormal replaces the normal appearance.  A rollover or down appearance
// which repeated the previous normal appearance repeats the new one
// afterwards, while one of its own is left alone.
//
// Code which fixes up or generates a normal appearance uses this instead of
// assigning to the fields, so that an annotation which looked the same under
// the pointer still does.  Where the dictionary may be shared, clone it first;
// see [Dict.Clone].
func (d *Dict) SetNormal(single *form.Form, byState map[pdf.Name]*form.Form) {
	rollOver := d.repeatsNormal(d.RollOver, d.RollOverMap)
	down := d.repeatsNormal(d.Down, d.DownMap)

	d.Normal, d.NormalMap = single, byState

	// the empty shorthand already follows the normal appearance
	if rollOver && (d.RollOver != nil || len(d.RollOverMap) > 0) {
		d.RollOver, d.RollOverMap = single, byState
	}
	if down && (d.Down != nil || len(d.DownMap) > 0) {
		d.Down, d.DownMap = single, byState
	}
}

// repeatsNormal reports whether the given appearance entry shows the same
// thing as the normal appearance, so that writing it out would only repeat
// /N.  An empty entry is the shorthand for the normal appearance.
func (d *Dict) repeatsNormal(single *form.Form, byState map[pdf.Name]*form.Form) bool {
	if single == nil && len(byState) == 0 {
		return true
	}
	return sameEntry(single, byState, d.Normal, d.NormalMap)
}

// sameEntry reports whether two entries of an appearance dictionary show the
// same thing.  Forms are compared by identity: two forms with equal content
// are still two appearance streams, and a file may well want both.
func sameEntry(single1 *form.Form, byState1 map[pdf.Name]*form.Form,
	single2 *form.Form, byState2 map[pdf.Name]*form.Form) bool {
	if single1 != nil || single2 != nil {
		return single1 == single2
	}
	if len(byState1) != len(byState2) {
		return false
	}
	for state, f := range byState1 {
		if g, ok := byState2[state]; !ok || g != f {
			return false
		}
	}
	return true
}

// embedEntry writes one entry of an appearance dictionary, either a single
// appearance stream or a stream per appearance state.
func embedEntry(e *pdf.EmbedHelper, single *form.Form, byState map[pdf.Name]*form.Form) (pdf.Object, error) {
	if single != nil {
		return e.Embed(single)
	}

	res := pdf.Dict{}
	for state, f := range byState {
		// an annotation selects its appearance with the AS entry, which an
		// empty name cannot fill, so such a state could never be shown
		if state == "" {
			return nil, errors.New("empty appearance state name")
		}
		obj, err := e.Embed(f)
		if err != nil {
			return nil, err
		}
		res[state] = obj
	}
	return res, nil
}

// HasDicts reports whether any appearance uses a state-dependent map.
func (d *Dict) HasDicts() bool {
	if d == nil {
		return false
	}

	return len(d.NormalMap) > 0 ||
		len(d.RollOverMap) > 0 ||
		len(d.DownMap) > 0
}

// AnyState returns a state name which selects an appearance, or the empty
// name if the dictionary does not give an appearance per state.  It is
// nil-safe: a nil dictionary yields the empty name.
//
// This serves code which has to supply an annotation's appearance state: a
// dictionary with an appearance per state is meaningless without one, and an
// annotation which named no state could not be written back.  The result is
// therefore usable as the state directly, an empty result standing for an
// annotation which needs no state.
//
// The normal appearance decides, since it is what the annotation shows at
// rest; the other entries only get a say where the normal appearance is a
// single stream.  The smallest name is returned, so that the choice does not
// follow map iteration order.
func (d *Dict) AnyState() pdf.Name {
	if d == nil {
		return ""
	}

	for _, byState := range []map[pdf.Name]*form.Form{
		d.NormalMap, d.RollOverMap, d.DownMap,
	} {
		var best pdf.Name
		for state := range byState {
			// an empty name cannot fill an AS entry, so it selects nothing
			if state == "" {
				continue
			}
			if best == "" || state < best {
				best = state
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}
