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

// Package iconannot builds the annotations a probe of annotation icons
// needs: one of each type which is displayed as an icon, showing each of the
// icons the specification names for it.
//
// A file attachment must carry a file and a sound annotation a sound, which
// have nothing to do with the icon but without which the annotation cannot
// be written; this package supplies both.
package iconannot

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/sound"
)

// Size is the side of the square an icon is drawn in.  The types which show
// one pin their rectangle to a square this size.
const Size = 24.0

// Group is one annotation type together with the icons the specification
// names for it.
type Group struct {
	Kind  string
	Icons []string
}

// Groups lists every annotation type which is displayed as an icon, with its
// icons in the order the specification gives them.  Rubber stamps are left
// out: they are named by the same /Name entry but drawn as a label rather
// than as an icon.
var Groups = []Group{
	{"Text", []string{
		"Comment", "Key", "Note", "Help", "NewParagraph", "Paragraph", "Insert",
	}},
	{"FileAttachment", []string{"Graph", "PushPin", "Paperclip", "Tag"}},
	{"Sound", []string{"Speaker", "Mic"}},
}

// New returns an annotation of the given kind showing the named icon, with
// its top-left corner at x, y.  The icon types pin their rectangle to a
// [Size] by [Size] square there.
//
// The caller passes the entries it wants to vary in common; the rectangle,
// the contents and the print flag are filled in here.
func New(kind, icon string, common annotation.Common, x, y float64) annotation.Annotation {
	common.Rect = pdf.Rectangle{
		LLx: x, LLy: y - Size,
		URx: x + Size, URy: y,
	}
	common.Contents = kind + " " + icon
	common.Flags |= annotation.FlagPrint

	markup := annotation.Markup{User: "Test User"}

	switch kind {
	case "FileAttachment":
		return &annotation.FileAttachment{
			Common: common,
			Markup: markup,
			Icon:   annotation.FileAttachmentIcon(icon),
			FS:     File(icon + ".txt"),
		}
	case "Sound":
		return &annotation.Sound{
			Common: common,
			Markup: markup,
			Icon:   annotation.SoundIcon(icon),
			Sound:  Silence(),
		}
	default:
		return &annotation.Text{
			Common: common,
			Markup: markup,
			Icon:   annotation.TextIcon(icon),
		}
	}
}

// File returns the file a file attachment annotation needs to carry.
func File(name string) *file.Specification {
	content := fmt.Sprintf("Sample content for %s\n", name)
	stream := &file.Stream{
		MimeType: "text/plain",
		Size:     int64(len(content)),
		WriteData: func(w io.Writer) error {
			_, err := w.Write([]byte(content))
			return err
		},
	}
	return &file.Specification{
		FileName:        name,
		FileNameUnicode: name,
		EmbeddedFiles:   map[string]*file.Stream{"F": stream, "UF": stream},
	}
}

// Silence returns the sound a sound annotation needs to carry: a moment of
// silence, since these pages are about the icon rather than the sound.
func Silence() *sound.Sound {
	return &sound.Sound{
		SampleRate:    11025,
		Channels:      1,
		BitsPerSample: 8,
		Encoding:      sound.EncodingRaw,
		Data: &sound.InlineSource{
			WriteData: func(w io.Writer) error {
				_, err := w.Write(make([]byte, 256))
				return err
			},
		},
	}
}
