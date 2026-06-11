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

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/sound"
)

// PDF 2.0 sections: 12.5.2 12.5.6.2 12.5.6.16

// Sound represents a sound annotation that contains sound recorded from the
// computer's microphone or imported from a file. When the annotation is
// activated, the sound is played. The annotation behaves like a text
// annotation in most ways, with a different icon (by default, a speaker) to
// indicate that it represents a sound.
//
// NOTE: Sound annotations are deprecated in PDF 2.0 and superseded by the
// general multimedia framework.
type Sound struct {
	Common
	Markup

	// Sound (required) is the sound object defining the sound that is
	// played when the annotation is activated.
	Sound *sound.Sound

	// Icon is the name of an icon that is used in displaying the annotation.
	// The standard icon names are Speaker and Mic.  Viewers may support
	// additional, application-specific names.
	//
	// When writing annotations, an empty Icon name can be used as a shorthand
	// for [SoundIconSpeaker].
	//
	// This corresponds to the /Name entry in the PDF annotation dictionary.
	Icon SoundIcon
}

var _ Annotation = (*Sound)(nil)

// AnnotationType returns "Sound".
// This implements the [Annotation] interface.
func (s *Sound) AnnotationType() pdf.Name {
	return "Sound"
}

func (s *Sound) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "sound annotation", pdf.V1_2); err != nil {
		return nil, err
	}
	if s.Sound == nil {
		return nil, errors.New("sound annotation must have a Sound object")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Sound"),
	}

	// Add common annotation fields
	if err := s.Common.fillDict(rm, dict, isMarkup(s), false); err != nil {
		return nil, err
	}

	// Add markup annotation fields
	if err := s.Markup.fillDict(rm, dict); err != nil {
		return nil, err
	}

	// Sound (required)
	soundObj, err := rm.Embed(s.Sound)
	if err != nil {
		return nil, err
	}
	dict["Sound"] = soundObj

	// Name (optional) - only write if not the default value "Speaker"
	if s.Icon != "" && s.Icon != SoundIconSpeaker {
		dict["Name"] = pdf.Name(s.Icon)
	}

	return dict, nil
}

// SoundIcon represents the name of an icon used to represent a sound annotation.
// The standard names defined by the PDF specification are provided as constants.
// Other names may be used, but support is viewer dependent.
type SoundIcon pdf.Name

// Standard PDF icon names for sound annotations.
const (
	// SoundIconSpeaker indicates a generic sound playback annotation.
	// Typically appears as a loudspeaker icon in PDF viewers.
	SoundIconSpeaker SoundIcon = "Speaker"

	// SoundIconMic indicates a recorded or microphone-sourced sound annotation.
	// Typically appears as a microphone icon in PDF viewers.
	SoundIconMic SoundIcon = "Mic"
)
