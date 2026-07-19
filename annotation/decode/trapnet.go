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

package decode

import (
	"maps"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/trapnet"
)

func decodeTrapNet(c pdf.Cursor, dict pdf.Dict) (*annotation.TrapNet, error) {
	trapNet := &annotation.TrapNet{}

	if err := decodeCommon(c, &trapNet.Common, dict); err != nil {
		return nil, err
	}

	// flag values for trap network annotations are prescribed by the spec:
	trapNet.Flags = annotation.FlagPrint | annotation.FlagReadOnly

	// Extract trap network-specific fields

	if lastModified, err := pdf.Optional(c.Date(dict["LastModified"])); err != nil {
		return nil, err
	} else {
		trapNet.LastModified = time.Time(lastModified)
	}

	if version, err := pdf.Optional(c.Array(dict["Version"])); err != nil {
		return nil, err
	} else if len(version) > 0 {
		refs := make([]pdf.Reference, 0, len(version))
		for _, obj := range version {
			if ref, ok := obj.(pdf.Reference); ok {
				refs = append(refs, ref)
			}
		}
		if len(refs) > 0 {
			trapNet.Version = refs
		}
	}

	if annotStates, err := pdf.Optional(c.Array(dict["AnnotStates"])); err != nil {
		return nil, err
	} else if len(annotStates) > 0 {
		states := make([]pdf.Name, len(annotStates))
		for i, obj := range annotStates {
			if name, ok := obj.(pdf.Name); ok {
				states[i] = name
			} else if obj == nil {
				states[i] = "" // null entry
			}
		}
		trapNet.AnnotStates = states
	}

	if fontFauxing, err := pdf.Optional(c.Array(dict["FontFauxing"])); err != nil {
		return nil, err
	} else if len(fontFauxing) > 0 {
		refs := make([]pdf.Reference, 0, len(fontFauxing))
		for _, obj := range fontFauxing {
			if ref, ok := obj.(pdf.Reference); ok {
				refs = append(refs, ref)
			}
		}
		if len(refs) > 0 {
			trapNet.FontFauxing = refs
		}
	}

	// repair field combinations
	hasLM := !trapNet.LastModified.IsZero()
	hasVer := len(trapNet.Version) > 0
	hasAS := len(trapNet.AnnotStates) > 0
	switch {
	case hasLM && !hasVer && !hasAS:
		// valid: LastModified only
	case !hasLM && hasVer && hasAS:
		// valid: Version + AnnotStates
	case hasLM && hasVer && hasAS:
		// all present: prefer Version+AnnotStates
		trapNet.LastModified = time.Time{}
	case hasLM:
		// LastModified + incomplete pair: keep LastModified, drop the rest
		trapNet.Version = nil
		trapNet.AnnotStates = nil
	default:
		// no valid combination: set LastModified, drop incomplete pair
		trapNet.LastModified = time.Unix(0, 0).UTC()
		trapNet.Version = nil
		trapNet.AnnotStates = nil
	}

	// A trap network's normal appearance is the trap network itself, so a bare
	// form is not enough here and the generic repair in [Annotation] cannot do
	// the job.  Supply the appearance and shape it in one place instead.
	if trapNet.Appearance == nil &&
		annotation.AppearanceRequired("TrapNet", trapNet.Rect, pdf.GetVersion(c.Getter())) {
		trapNet.Appearance = emptyAppearance(trapNet.Rect)
	}
	trapNet.Appearance = repairTrapNetAppearance(trapNet.Appearance)

	return trapNet, nil
}

// repairTrapNetAppearance makes sure the normal appearance of a trap network
// annotation is a valid trap network, so that it can be written back.
//
// Appearance dictionaries and the forms inside them are cached by the
// extractor and can be shared with other annotations, so copies are made
// whenever something needs fixing.  ap is returned unchanged otherwise.
func repairTrapNetAppearance(ap *appearance.Dict) *appearance.Dict {
	if ap == nil {
		return nil
	}

	normal := asTrapNet(ap.Normal)

	normalMap := ap.NormalMap
	copied := false
	for name, f := range ap.NormalMap {
		repaired := asTrapNet(f)
		if repaired == f {
			continue
		}
		if !copied {
			normalMap = maps.Clone(ap.NormalMap)
			copied = true
		}
		normalMap[name] = repaired
	}

	if normal == ap.Normal && !copied {
		return ap
	}

	fixed := *ap
	fixed.Normal = normal
	fixed.NormalMap = normalMap
	return &fixed
}

// asTrapNet returns f with the trap network entries supplied if they are
// missing, and f itself otherwise.  Other entries are left alone: the trap
// network entries take effect because of where the form is referenced from,
// and a form which also carries printer's mark entries is not thereby invalid.
func asTrapNet(f *form.Form) *form.Form {
	if f == nil || f.TrapNet != nil {
		return f
	}
	clone := *f
	clone.TrapNet = &trapnet.Attributes{PCM: trapnet.DefaultPCM}
	return &clone
}
