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
	"seehuhn.de/go/pdf/optional"
)

// MediaCriteria specifies a set of criteria that shall be met for a rendition
// to be viable.
type MediaCriteria struct {
	// AudioDescriptions, if set, must match the user's preference for hearing
	// audio descriptions.
	AudioDescriptions optional.Bool

	// Captions, if set, must match the user's preference for seeing text
	// captions.
	Captions optional.Bool

	// Overdubs, if set, must match the user's preference for hearing audio
	// overdubs.
	Overdubs optional.Bool

	// Subtitles, if set, must match the user's preference for seeing
	// subtitles.
	Subtitles optional.Bool

	// Bandwidth, if set, is the minimum system bandwidth in bits per second.
	Bandwidth optional.UInt

	// MinBitDepth (optional) is the minimum monitor colour depth.
	MinBitDepth *MinBitDepth

	// MinScreenSize (optional) is the minimum monitor screen size.
	MinScreenSize *MinScreenSize

	// Software (optional), if non-empty, lists the software the interactive
	// PDF processor must be identified by.
	Software []*SoftwareIdentifier

	// Version (optional), if non-empty, holds a minimum and optionally a
	// maximum PDF language version, in the format of the catalog Version
	// entry.
	Version []pdf.Name

	// Languages (optional), if non-empty, lists the languages the interactive
	// PDF processor may be running in.
	Languages []string

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractMediaCriteria reads a media criteria dictionary.
func ExtractMediaCriteria(c pdf.Cursor, obj pdf.Object, isDirect bool) (*MediaCriteria, error) {
	dict, err := c.DictTyped(obj, "MediaCriteria")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media criteria dictionary")
	}

	crit := &MediaCriteria{SingleUse: isDirect}

	for key, field := range map[pdf.Name]*optional.Bool{
		"A": &crit.AudioDescriptions,
		"C": &crit.Captions,
		"O": &crit.Overdubs,
		"S": &crit.Subtitles,
	} {
		if dict[key] == nil {
			continue
		}
		if b, err := pdf.Optional(c.Boolean(dict[key])); err != nil {
			return nil, err
		} else {
			field.Set(bool(b))
		}
	}

	if dict["R"] != nil {
		if r, err := pdf.Optional(c.Integer(dict["R"])); err != nil {
			return nil, err
		} else if r >= 0 {
			crit.Bandwidth.Set(uint(r))
		}
	}

	if d, err := pdf.DecodeOptional(c, dict["D"], ExtractMinBitDepth); err != nil {
		return nil, err
	} else {
		crit.MinBitDepth = d
	}
	if z, err := pdf.DecodeOptional(c, dict["Z"], ExtractMinScreenSize); err != nil {
		return nil, err
	} else {
		crit.MinScreenSize = z
	}

	if v, err := pdf.Optional(c.Array(dict["V"])); err != nil {
		return nil, err
	} else {
		for _, elem := range v {
			id, err := pdf.DecodeOptional(c, elem, ExtractSoftwareIdentifier)
			if err != nil {
				return nil, err
			}
			if id != nil {
				crit.Software = append(crit.Software, id)
			}
		}
	}

	if p, err := pdf.Optional(c.Array(dict["P"])); err != nil {
		return nil, err
	} else if len(p) == 1 || len(p) == 2 {
		ok := true
		ver := make([]pdf.Name, 0, len(p))
		for _, elem := range p {
			name, err := pdf.Optional(c.Name(elem))
			if err != nil {
				return nil, err
			} else if name == "" {
				ok = false
				break
			}
			ver = append(ver, name)
		}
		if ok {
			crit.Version = ver
		}
	}

	if l, err := pdf.Optional(c.Array(dict["L"])); err != nil {
		return nil, err
	} else {
		for _, elem := range l {
			lang, err := pdf.Optional(c.TextString(elem))
			if err != nil {
				return nil, err
			}
			crit.Languages = append(crit.Languages, string(lang))
		}
	}

	return crit, nil
}

// Embed converts the media criteria to its PDF representation.
func (c *MediaCriteria) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media criteria", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaCriteria")
	}

	for key, field := range map[pdf.Name]optional.Bool{
		"A": c.AudioDescriptions,
		"C": c.Captions,
		"O": c.Overdubs,
		"S": c.Subtitles,
	} {
		if v, ok := field.Get(); ok {
			dict[key] = pdf.Boolean(v)
		}
	}

	if r, ok := c.Bandwidth.Get(); ok {
		dict["R"] = pdf.Integer(r)
	}

	if c.MinBitDepth != nil {
		d, err := e.Embed(c.MinBitDepth)
		if err != nil {
			return nil, err
		}
		dict["D"] = d
	}
	if c.MinScreenSize != nil {
		z, err := e.Embed(c.MinScreenSize)
		if err != nil {
			return nil, err
		}
		dict["Z"] = z
	}

	if len(c.Software) > 0 {
		arr := make(pdf.Array, len(c.Software))
		for i, id := range c.Software {
			obj, err := e.Embed(id)
			if err != nil {
				return nil, err
			}
			arr[i] = obj
		}
		dict["V"] = arr
	}

	if len(c.Version) > 0 {
		if len(c.Version) > 2 {
			return nil, pdf.Error("media criteria: Version must have one or two elements")
		}
		arr := make(pdf.Array, len(c.Version))
		for i, name := range c.Version {
			arr[i] = name
		}
		dict["P"] = arr
	}

	if len(c.Languages) > 0 {
		arr := make(pdf.Array, len(c.Languages))
		for i, lang := range c.Languages {
			arr[i] = pdf.TextString(lang)
		}
		dict["L"] = arr
	}

	if c.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// MinBitDepth specifies the minimum monitor colour depth required for a
// rendition to be viable.
type MinBitDepth struct {
	// Depth is the minimum screen depth in bits.  It must be positive.
	Depth int

	// Monitor specifies which monitor the depth is tested against.
	Monitor MonitorSpecifier

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractMinBitDepth reads a minimum bit depth dictionary.
func ExtractMinBitDepth(c pdf.Cursor, obj pdf.Object, isDirect bool) (*MinBitDepth, error) {
	dict, err := c.DictTyped(obj, "MinBitDepth")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing minimum bit depth dictionary")
	}

	v, err := pdf.Optional(c.Integer(dict["V"]))
	if err != nil {
		return nil, err
	} else if v <= 0 {
		return nil, pdf.Error("invalid minimum bit depth")
	}

	d := &MinBitDepth{Depth: int(v), SingleUse: isDirect}
	if m, err := pdf.Optional(c.Integer(dict["M"])); err != nil {
		return nil, err
	} else if MonitorSpecifier(m).isValid() {
		d.Monitor = MonitorSpecifier(m)
	}

	return d, nil
}

// Embed converts the minimum bit depth to its PDF representation.
func (d *MinBitDepth) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "minimum bit depth", pdf.V1_5); err != nil {
		return nil, err
	}
	if d.Depth <= 0 {
		return nil, pdf.Error("minimum bit depth: Depth must be positive")
	}
	if !d.Monitor.isValid() {
		return nil, pdf.Error("minimum bit depth: invalid Monitor")
	}

	dict := pdf.Dict{
		"V": pdf.Integer(d.Depth),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MinBitDepth")
	}
	if d.Monitor != MonitorLargestDocument {
		dict["M"] = pdf.Integer(d.Monitor)
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

// MinScreenSize specifies the minimum monitor screen size required for a
// rendition to be viable.
type MinScreenSize struct {
	// Width is the minimum monitor width in pixels.
	Width int

	// Height is the minimum monitor height in pixels.
	Height int

	// Monitor specifies which monitor the size is tested against.
	Monitor MonitorSpecifier

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractMinScreenSize reads a minimum screen size dictionary.
func ExtractMinScreenSize(c pdf.Cursor, obj pdf.Object, isDirect bool) (*MinScreenSize, error) {
	dict, err := c.DictTyped(obj, "MinScreenSize")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing minimum screen size dictionary")
	}

	arr, err := pdf.Optional(c.Array(dict["V"]))
	if err != nil {
		return nil, err
	} else if len(arr) < 2 {
		return nil, pdf.Error("invalid minimum screen size")
	}
	w, errW := pdf.Optional(c.Integer(arr[0]))
	h, errH := pdf.Optional(c.Integer(arr[1]))
	if errW != nil {
		return nil, errW
	}
	if errH != nil {
		return nil, errH
	}
	if w < 0 || h < 0 {
		return nil, pdf.Error("invalid minimum screen size")
	}

	s := &MinScreenSize{Width: int(w), Height: int(h), SingleUse: isDirect}
	if m, err := pdf.Optional(c.Integer(dict["M"])); err != nil {
		return nil, err
	} else if MonitorSpecifier(m).isValid() {
		s.Monitor = MonitorSpecifier(m)
	}

	return s, nil
}

// Embed converts the minimum screen size to its PDF representation.
func (s *MinScreenSize) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "minimum screen size", pdf.V1_5); err != nil {
		return nil, err
	}
	if s.Width < 0 || s.Height < 0 {
		return nil, pdf.Error("minimum screen size: dimensions must not be negative")
	}
	if !s.Monitor.isValid() {
		return nil, pdf.Error("minimum screen size: invalid Monitor")
	}

	dict := pdf.Dict{
		"V": pdf.Array{pdf.Integer(s.Width), pdf.Integer(s.Height)},
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MinScreenSize")
	}
	if s.Monitor != MonitorLargestDocument {
		dict["M"] = pdf.Integer(s.Monitor)
	}

	if s.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
