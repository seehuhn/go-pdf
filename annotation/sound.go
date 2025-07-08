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

package annotation

import "seehuhn.de/go/pdf"

// Sound represents a sound annotation that contains sound recorded from the
// computer's microphone or imported from a file. When the annotation is
// activated, the sound is played. The annotation behaves like a text annotation
// in most ways, with a different icon (by default, a speaker) to indicate that
// it represents a sound.
//
// NOTE: Sound annotations are deprecated in PDF 2.0 and superseded by the
// general multimedia framework.
type Sound struct {
	Common
	Markup

	// Sound (required) is a sound object defining the sound that is
	// played when the annotation is activated.
	Sound pdf.Reference

	// Name (optional) is the name of an icon that is used in displaying
	// the annotation. Standard names include:
	// Speaker, Mic
	// Default value: "Speaker"
	Name pdf.Name
}

var _ pdf.Annotation = (*Sound)(nil)

// AnnotationType returns "Sound".
// This implements the [pdf.Annotation] interface.
func (s *Sound) AnnotationType() pdf.Name {
	return "Sound"
}

func extractSound(r pdf.Getter, dict pdf.Dict) (*Sound, error) {
	sound := &Sound{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &sound.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &sound.Markup); err != nil {
		return nil, err
	}

	// Extract sound-specific fields
	// Sound (required)
	if soundRef, ok := dict["Sound"].(pdf.Reference); ok {
		sound.Sound = soundRef
	}

	// Name (optional) - default to "Speaker" if not specified
	if name, err := pdf.GetName(r, dict["Name"]); err == nil && name != "" {
		sound.Name = name
	} else {
		sound.Name = "Speaker" // PDF default value
	}

	return sound, nil
}

func (s *Sound) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "sound annotation", pdf.V1_2); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Sound"),
	}

	// Add common annotation fields
	if err := s.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add markup annotation fields
	if err := s.Markup.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add sound-specific fields
	// Sound (required)
	if s.Sound != 0 {
		dict["Sound"] = s.Sound
	}

	// Name (optional) - only write if not the default value "Speaker"
	if s.Name != "" && s.Name != "Speaker" {
		dict["Name"] = s.Name
	}

	return dict, zero, nil
}
