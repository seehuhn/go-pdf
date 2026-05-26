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

// Rendition selects and configures the media to be played by a rendition
// action.  It is implemented by [MediaRendition] and [SelectorRendition].
type Rendition interface {
	pdf.Embedder
	isRendition()
}

func (*MediaRendition) isRendition()    {}
func (*SelectorRendition) isRendition() {}

// RenditionCommon holds the entries common to all rendition dictionaries.
type RenditionCommon struct {
	// Name (optional) is the name of the rendition, for use in a user
	// interface and for name tree lookup by ECMAScript actions.
	Name string

	// MustHonourCriteria (optional) are the criteria that must be met for the
	// rendition to be viable.
	MustHonourCriteria *MediaCriteria

	// BestEffortCriteria (optional) are the criteria that should be met on a
	// best-effort basis.
	BestEffortCriteria *MediaCriteria
}

func extractRenditionCommon(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, c *RenditionCommon) error {
	if n, err := pdf.Optional(pdf.GetTextString(x.R, dict["N"])); err != nil {
		return err
	} else {
		c.Name = string(n)
	}

	var err error
	if c.MustHonourCriteria, err = extractCriteria(x, path, dict["MH"]); err != nil {
		return err
	}
	if c.BestEffortCriteria, err = extractCriteria(x, path, dict["BE"]); err != nil {
		return err
	}
	return nil
}

func extractCriteria(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) (*MediaCriteria, error) {
	dict, err := pdf.Optional(x.GetDict(path, obj))
	if err != nil || dict == nil {
		return nil, err
	}
	return pdf.ExtractorGetOptional(x, path, dict["C"], ExtractMediaCriteria)
}

func (c *RenditionCommon) fillDict(e *pdf.EmbedHelper, dict pdf.Dict) error {
	if c.Name != "" {
		dict["N"] = pdf.TextString(c.Name)
	}
	if c.MustHonourCriteria != nil {
		crit, err := e.Embed(c.MustHonourCriteria)
		if err != nil {
			return err
		}
		dict["MH"] = pdf.Dict{"C": crit}
	}
	if c.BestEffortCriteria != nil {
		crit, err := e.Embed(c.BestEffortCriteria)
		if err != nil {
			return err
		}
		dict["BE"] = pdf.Dict{"C": crit}
	}
	return nil
}

// ExtractRendition reads a rendition dictionary and dispatches on its subtype.
func ExtractRendition(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (Rendition, error) {
	dict, err := x.GetDictTyped(path, obj, "Rendition")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing rendition dictionary")
	}

	s, err := pdf.Optional(x.GetName(path, dict["S"]))
	if err != nil {
		return nil, err
	}

	switch s {
	case "MR":
		return extractMediaRendition(x, path, dict, isDirect)
	case "SR":
		return extractSelectorRendition(x, path, dict, isDirect)
	default:
		return nil, pdf.Error("unknown rendition subtype: " + string(s))
	}
}

// MediaRendition specifies what media to play, and how and where to play it.
type MediaRendition struct {
	RenditionCommon

	// Clip (optional) specifies what should be played.
	Clip MediaClip

	// Play (optional) specifies how the media should be played.
	Play *MediaPlayParameters

	// Screen (optional) specifies where the media should be played.
	Screen *MediaScreenParameters

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

func extractMediaRendition(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, isDirect bool) (*MediaRendition, error) {
	r := &MediaRendition{SingleUse: isDirect}
	if err := extractRenditionCommon(x, path, dict, &r.RenditionCommon); err != nil {
		return nil, err
	}

	var err error
	if r.Clip, err = pdf.ExtractorGetOptional(x, path, dict["C"], ExtractMediaClip); err != nil {
		return nil, err
	}
	if r.Play, err = pdf.ExtractorGetOptional(x, path, dict["P"], ExtractMediaPlayParameters); err != nil {
		return nil, err
	}
	if r.Screen, err = pdf.ExtractorGetOptional(x, path, dict["SP"], ExtractMediaScreenParameters); err != nil {
		return nil, err
	}

	return r, nil
}

// Embed converts the media rendition to its PDF representation.
func (r *MediaRendition) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media rendition", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name("MR"),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Rendition")
	}
	if err := r.RenditionCommon.fillDict(e, dict); err != nil {
		return nil, err
	}

	if r.Clip != nil {
		c, err := e.Embed(r.Clip)
		if err != nil {
			return nil, err
		}
		dict["C"] = c
	}
	if r.Play != nil {
		p, err := e.Embed(r.Play)
		if err != nil {
			return nil, err
		}
		dict["P"] = p
	}
	if r.Screen != nil {
		sp, err := e.Embed(r.Screen)
		if err != nil {
			return nil, err
		}
		dict["SP"] = sp
	}

	if r.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// SelectorRendition contains an ordered list of renditions, of which the first
// viable media rendition should be played.
type SelectorRendition struct {
	RenditionCommon

	// Renditions is the ordered list of candidate renditions.  An empty list
	// is valid.
	Renditions []Rendition

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

func extractSelectorRendition(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, isDirect bool) (*SelectorRendition, error) {
	r := &SelectorRendition{SingleUse: isDirect}
	if err := extractRenditionCommon(x, path, dict, &r.RenditionCommon); err != nil {
		return nil, err
	}

	arr, err := pdf.Optional(x.GetArray(path, dict["R"]))
	if err != nil {
		return nil, err
	}
	for _, elem := range arr {
		child, err := pdf.ExtractorGetOptional(x, path, elem, ExtractRendition)
		if err != nil {
			return nil, err
		}
		if child != nil {
			r.Renditions = append(r.Renditions, child)
		}
	}

	return r, nil
}

// Embed converts the selector rendition to its PDF representation.
func (r *SelectorRendition) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "selector rendition", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"S": pdf.Name("SR"),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Rendition")
	}
	if err := r.RenditionCommon.fillDict(e, dict); err != nil {
		return nil, err
	}

	arr := make(pdf.Array, len(r.Renditions))
	for i, child := range r.Renditions {
		if child == nil {
			return nil, pdf.Error("selector rendition: nil entry in Renditions")
		}
		obj, err := e.Embed(child)
		if err != nil {
			return nil, err
		}
		arr[i] = obj
	}
	dict["R"] = arr

	if r.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
