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

// MediaPlayers specifies which players may or may not be used to play a media
// object.
type MediaPlayers struct {
	// MustUse, if non-empty, lists players one of which must be used.
	MustUse []*MediaPlayerInfo

	// Allowed, if non-empty, lists players any of which may be used.  It is
	// ignored if MustUse is non-empty.
	Allowed []*MediaPlayerInfo

	// NotUsed, if non-empty, lists players that must not be used.
	NotUsed []*MediaPlayerInfo

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractMediaPlayers reads a media players dictionary.
func ExtractMediaPlayers(c pdf.Cursor, obj pdf.Object, isDirect bool) (*MediaPlayers, error) {
	dict, err := c.DictTyped(obj, "MediaPlayers")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media players dictionary")
	}

	p := &MediaPlayers{SingleUse: isDirect}

	if p.MustUse, err = extractPlayerInfoArray(c, dict["MU"]); err != nil {
		return nil, err
	}
	if p.Allowed, err = extractPlayerInfoArray(c, dict["A"]); err != nil {
		return nil, err
	}
	if p.NotUsed, err = extractPlayerInfoArray(c, dict["NU"]); err != nil {
		return nil, err
	}

	return p, nil
}

func extractPlayerInfoArray(c pdf.Cursor, obj pdf.Object) ([]*MediaPlayerInfo, error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return nil, err
	}
	var out []*MediaPlayerInfo
	for _, elem := range arr {
		info, err := pdf.DecodeOptional(c, elem, ExtractMediaPlayerInfo)
		if err != nil {
			return nil, err
		}
		if info != nil {
			out = append(out, info)
		}
	}
	return out, nil
}

// Embed converts the media players dictionary to its PDF representation.
func (p *MediaPlayers) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media players", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaPlayers")
	}

	for key, infos := range map[pdf.Name][]*MediaPlayerInfo{
		"MU": p.MustUse,
		"A":  p.Allowed,
		"NU": p.NotUsed,
	} {
		if len(infos) == 0 {
			continue
		}
		arr := make(pdf.Array, len(infos))
		for i, info := range infos {
			obj, err := e.Embed(info)
			if err != nil {
				return nil, err
			}
			arr[i] = obj
		}
		dict[key] = arr
	}

	if p.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// MediaPlayerInfo provides information about a specific media player.
type MediaPlayerInfo struct {
	// PID identifies the player by name, version range and operating systems.
	PID *SoftwareIdentifier

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractMediaPlayerInfo reads a media player info dictionary.
func ExtractMediaPlayerInfo(c pdf.Cursor, obj pdf.Object, isDirect bool) (*MediaPlayerInfo, error) {
	dict, err := c.DictTyped(obj, "MediaPlayerInfo")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing media player info dictionary")
	}

	pid, err := pdf.Decode(c, dict["PID"], ExtractSoftwareIdentifier)
	if err != nil {
		return nil, err
	} else if pid == nil {
		return nil, pdf.Error("media player info missing PID entry")
	}

	return &MediaPlayerInfo{PID: pid, SingleUse: isDirect}, nil
}

// Embed converts the media player info to its PDF representation.
func (info *MediaPlayerInfo) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "media player info", pdf.V1_5); err != nil {
		return nil, err
	}
	if info.PID == nil {
		return nil, pdf.Error("media player info: PID is required")
	}

	pid, err := e.Embed(info.PID)
	if err != nil {
		return nil, err
	}
	dict := pdf.Dict{
		"PID": pid,
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("MediaPlayerInfo")
	}

	if info.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
