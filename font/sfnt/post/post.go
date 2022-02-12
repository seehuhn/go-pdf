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

// Package post has code for reading and wrinting the "post" table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/post
package post

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Info contains information from the "post" table.
type Info struct {
	ItalicAngle        int32 // Italic angle in degrees
	UnderlinePosition  int16 // Underline position (negative)
	UnderlineThickness int16 // Underline thickness
	IsFixedPitch       bool
}

// Read reads the "post" table from r.
func Read(r io.Reader) (*Info, error) {
	post := &postEnc{}
	if err := binary.Read(r, binary.BigEndian, post); err != nil {
		return nil, err
	}
	if post.Version != 0x00010000 && post.Version != 0x00020000 &&
		post.Version != 0x00025000 && post.Version != 0x00030000 {
		return nil, fmt.Errorf("post: unsupported version %08x", post.Version)
	}

	info := &Info{
		ItalicAngle:        post.ItalicAngle,
		UnderlinePosition:  post.UnderlinePosition,
		UnderlineThickness: post.UnderlineThickness,
		IsFixedPitch:       post.IsFixedPitch != 0,
	}

	return info, nil
}

// Encode encodes the "post" table.
func (info *Info) Encode() []byte {
	var isFixedPitch uint32
	if info.IsFixedPitch {
		isFixedPitch = 1
	}

	post := &postEnc{
		Version:            0x00030000,
		ItalicAngle:        info.ItalicAngle,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsFixedPitch:       isFixedPitch,
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, post)
	return buf.Bytes()
}

type postEnc struct {
	Version            uint32
	ItalicAngle        int32
	UnderlinePosition  int16
	UnderlineThickness int16
	IsFixedPitch       uint32
	MinMemType42       uint32
	MaxMemType42       uint32
	MinMemType1        uint32
	MaxMemType1        uint32
}
