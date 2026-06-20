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
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
)

// MediaClip specifies what should be played by a media rendition.  It is
// implemented by [MediaClipData] and [MediaClipSection].
type MediaClip interface {
	pdf.Embedder
	isMediaClip()
}

func (*MediaClipData) isMediaClip()    {}
func (*MediaClipSection) isMediaClip() {}

// ExtractMediaClip reads a media clip dictionary and dispatches on its
// subtype.
func ExtractMediaClip(c pdf.Cursor, obj pdf.Object, isDirect bool) (MediaClip, error) {
	dict, err := c.DictTyped(obj, "MediaClip")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media clip dictionary")
	}

	s, err := pdf.Optional(c.Name(dict["S"]))
	if err != nil {
		return nil, err
	}

	switch s {
	case "MCD":
		return extractMediaClipData(c, dict, isDirect)
	case "MCS":
		return extractMediaClipSection(c, dict, isDirect)
	default:
		return nil, pdf.Error("unknown media clip subtype: " + string(s))
	}
}

// MediaClipData defines the data for a media object that can be played.
type MediaClipData struct {
	// Name (optional) is the name of the media clip, for use in the user
	// interface.
	Name string

	// DataFile holds the media data as a file specification.  Exactly one of
	// DataFile and DataForm must be set.
	DataFile *file.Specification

	// DataForm holds the media data as a form XObject, played by the
	// interactive PDF processor itself.  Exactly one of DataFile and DataForm
	// must be set.
	DataForm *form.Form

	// ContentType (optional) identifies the type of data, following the
	// content type syntax of RFC 2045.  It must not be set for form XObjects.
	ContentType string

	// Permissions (optional) controls the use of the media data.
	Permissions *MediaPermissions

	// Alt (optional) provides alternative text descriptions for use when the
	// media cannot be played.
	Alt MultiLangText

	// Players (optional) identifies players that are valid and not valid for
	// playing the media.
	Players *MediaPlayers

	// MustHonourBaseURL, if non-empty, is the base URL that must be honoured
	// when resolving relative URLs in the media data.
	MustHonourBaseURL string

	// BestEffortBaseURL, if non-empty, is the base URL that should be honoured
	// on a best-effort basis.
	BestEffortBaseURL string

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

func extractMediaClipData(c pdf.Cursor, dict pdf.Dict, isDirect bool) (*MediaClipData, error) {
	clip := &MediaClipData{SingleUse: isDirect}

	if n, err := pdf.Optional(c.TextString(dict["N"])); err != nil {
		return nil, err
	} else {
		clip.Name = string(n)
	}

	resolved, err := c.Resolve(dict["D"])
	if err != nil {
		return nil, err
	}
	switch resolved.(type) {
	case *pdf.Stream:
		f, err := pdf.Decode(c, dict["D"], extract.Form)
		if err != nil {
			return nil, err
		}
		clip.DataForm = f
	case pdf.Dict:
		spec, err := pdf.Decode(c, dict["D"], file.ExtractSpecification)
		if err != nil {
			return nil, err
		}
		clip.DataFile = spec
	}
	if clip.DataFile == nil && clip.DataForm == nil {
		return nil, pdf.Error("media clip data has no valid D entry")
	}

	if clip.DataFile != nil {
		if ct, err := pdf.Optional(c.String(dict["CT"])); err != nil {
			return nil, err
		} else {
			clip.ContentType = string(ct)
		}
	}

	if p, err := pdf.DecodeOptional(c, dict["P"], ExtractMediaPermissions); err != nil {
		return nil, err
	} else {
		clip.Permissions = p
	}

	if clip.Alt, err = extractMultiLangText(c, dict["Alt"]); err != nil {
		return nil, err
	}

	if pl, err := pdf.DecodeOptional(c, dict["PL"], ExtractMediaPlayers); err != nil {
		return nil, err
	} else {
		clip.Players = pl
	}

	clip.MustHonourBaseURL = extractBaseURL(c, dict["MH"])
	clip.BestEffortBaseURL = extractBaseURL(c, dict["BE"])

	return clip, nil
}

func extractBaseURL(c pdf.Cursor, obj pdf.Object) string {
	dict, err := pdf.Optional(c.Dict(obj))
	if err != nil || dict == nil {
		return ""
	}
	bu, _ := pdf.Optional(c.String(dict["BU"]))
	return string(bu)
}

// Embed converts the media clip data to its PDF representation.
func (c *MediaClipData) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media clip data", pdf.V1_5); err != nil {
		return nil, err
	}
	if (c.DataFile == nil) == (c.DataForm == nil) {
		return nil, pdf.Error("media clip data: exactly one of DataFile and DataForm must be set")
	}
	if c.DataForm != nil && c.ContentType != "" {
		return nil, pdf.Error("media clip data: ContentType must not be set for a form XObject")
	}

	dict := pdf.Dict{
		"S": pdf.Name("MCD"),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaClip")
	}
	if c.Name != "" {
		dict["N"] = pdf.TextString(c.Name)
	}

	var data pdf.Native
	var err error
	if c.DataFile != nil {
		data, err = e.Embed(c.DataFile)
	} else {
		data, err = e.Embed(c.DataForm)
	}
	if err != nil {
		return nil, err
	}
	dict["D"] = data

	if c.ContentType != "" {
		dict["CT"] = pdf.String(c.ContentType)
	}
	if c.Permissions != nil {
		p, err := e.Embed(c.Permissions)
		if err != nil {
			return nil, err
		}
		dict["P"] = p
	}
	if len(c.Alt) > 0 {
		dict["Alt"] = c.Alt.toArray()
	}
	if c.Players != nil {
		pl, err := e.Embed(c.Players)
		if err != nil {
			return nil, err
		}
		dict["PL"] = pl
	}
	if c.MustHonourBaseURL != "" {
		dict["MH"] = pdf.Dict{"BU": pdf.String(c.MustHonourBaseURL)}
	}
	if c.BestEffortBaseURL != "" {
		dict["BE"] = pdf.Dict{"BU": pdf.String(c.BestEffortBaseURL)}
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

// MediaClipSection defines a continuous section of another media clip object.
type MediaClipSection struct {
	// Name (optional) is the name of the media clip, for use in the user
	// interface.
	Name string

	// Next is the next-level media clip of which this section defines a
	// continuous part.
	Next MediaClip

	// Alt (optional) provides alternative text descriptions for use when the
	// media cannot be played.
	Alt MultiLangText

	// MustHonourBegin, MustHonourEnd (optional) are the begin and end offsets
	// that must be honoured.
	MustHonourBegin MediaOffset
	MustHonourEnd   MediaOffset

	// BestEffortBegin, BestEffortEnd (optional) are the begin and end offsets
	// that should be honoured on a best-effort basis.
	BestEffortBegin MediaOffset
	BestEffortEnd   MediaOffset

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

func extractMediaClipSection(c pdf.Cursor, dict pdf.Dict, isDirect bool) (*MediaClipSection, error) {
	sec := &MediaClipSection{SingleUse: isDirect}

	if n, err := pdf.Optional(c.TextString(dict["N"])); err != nil {
		return nil, err
	} else {
		sec.Name = string(n)
	}

	next, err := pdf.Decode(c, dict["D"], ExtractMediaClip)
	if err != nil {
		return nil, err
	} else if next == nil {
		return nil, pdf.Error("media clip section missing D entry")
	}
	sec.Next = next

	if sec.Alt, err = extractMultiLangText(c, dict["Alt"]); err != nil {
		return nil, err
	}

	sec.MustHonourBegin, sec.MustHonourEnd, err = extractSectionOffsets(c, dict["MH"])
	if err != nil {
		return nil, err
	}
	sec.BestEffortBegin, sec.BestEffortEnd, err = extractSectionOffsets(c, dict["BE"])
	if err != nil {
		return nil, err
	}

	return sec, nil
}

func extractSectionOffsets(c pdf.Cursor, obj pdf.Object) (begin, end MediaOffset, err error) {
	dict, err := pdf.Optional(c.Dict(obj))
	if err != nil || dict == nil {
		return nil, nil, err
	}
	if begin, err = pdf.DecodeOptional(c, dict["B"], ExtractMediaOffset); err != nil {
		return nil, nil, err
	}
	if end, err = pdf.DecodeOptional(c, dict["E"], ExtractMediaOffset); err != nil {
		return nil, nil, err
	}
	return begin, end, nil
}

// Embed converts the media clip section to its PDF representation.
func (c *MediaClipSection) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media clip section", pdf.V1_5); err != nil {
		return nil, err
	}
	if c.Next == nil {
		return nil, pdf.Error("media clip section: Next is required")
	}

	dict := pdf.Dict{
		"S": pdf.Name("MCS"),
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaClip")
	}
	if c.Name != "" {
		dict["N"] = pdf.TextString(c.Name)
	}

	next, err := e.Embed(c.Next)
	if err != nil {
		return nil, err
	}
	dict["D"] = next

	if len(c.Alt) > 0 {
		dict["Alt"] = c.Alt.toArray()
	}

	if mh, err := sectionOffsetsDict(e, c.MustHonourBegin, c.MustHonourEnd); err != nil {
		return nil, err
	} else if mh != nil {
		dict["MH"] = mh
	}
	if be, err := sectionOffsetsDict(e, c.BestEffortBegin, c.BestEffortEnd); err != nil {
		return nil, err
	} else if be != nil {
		dict["BE"] = be
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

func sectionOffsetsDict(e *pdf.EmbedHelper, begin, end MediaOffset) (pdf.Dict, error) {
	if begin == nil && end == nil {
		return nil, nil
	}
	dict := pdf.Dict{}
	if begin != nil {
		b, err := e.Embed(begin)
		if err != nil {
			return nil, err
		}
		dict["B"] = b
	}
	if end != nil {
		ee, err := e.Embed(end)
		if err != nil {
			return nil, err
		}
		dict["E"] = ee
	}
	return dict, nil
}
