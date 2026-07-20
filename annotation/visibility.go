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

package annotation

import (
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/oc"
)

// Resolve returns the annotation's appearance form of the given kind,
// honouring the appearance state (/AS).
func Resolve(c *Common, kind appearance.Kind) *form.Form {
	return c.Appearance.Resolve(c.AppearanceState, kind)
}

// HasAppearance reports whether an annotation has a usable normal appearance
// stream, i.e. one with content.  Callers that pre-populate fallback
// appearances should pair this with [ShouldSynthesizeFallback].
//
// An appearance without content does not count.  Reading an annotation which
// needs an appearance but has none supplies an empty one, so that the
// annotation can be written back; such an appearance draws nothing and must
// not suppress the fallback.
func HasAppearance(a Annotation) bool {
	ap := Resolve(a.GetCommon(), appearance.Normal)
	return ap != nil && ap.Content != nil
}

// HasRolloverAppearance reports whether an annotation has a rollover (/R)
// appearance stream with content.
func HasRolloverAppearance(a Annotation) bool {
	ap := Resolve(a.GetCommon(), appearance.RollOver)
	return ap != nil && ap.Content != nil
}

// ShouldSynthesizeFallback reports whether a fallback appearance may be
// generated for an annotation that has no appearance stream of its own.
//
// This is a question about display, not about what a file must contain.  An
// annotation excluded here may still be required to carry an appearance, and
// reading one supplies an empty appearance so that it stays writable; see
// [AppearanceRequired].
//
// The excluded types fall into two groups:
//
// A Screen with no AP "shall not have a default visual appearance and shall
// not be printed" (§12.5.6.18), so a reader must not invent one.
//
// For printer's marks, trap networks, popups and projections there is nothing
// to invent.  The visual presentation of a printer's mark is the form XObject
// in its N entry (§14.11.3), and a trap network likewise is the form XObject
// painting the traps (§14.11.6): with no appearance there is no mark and no
// trap network.  Drawing a substitute would put a registration target on a
// plate the file never specified, or paint over finished page content, since a
// trap network prints last.  A popup is the note window of its parent markup
// annotation rather than independent page content, and neither it nor a
// projection is required to carry an appearance at all.
func ShouldSynthesizeFallback(a Annotation) bool {
	switch a.(type) {
	case *Screen:
		return false
	case *PrinterMark, *TrapNet:
		return false
	case *Popup, *Projection:
		return false
	}
	return true
}

// EffectiveAnnotFlags returns the annotation's flags including the implicit
// NoZoom and NoRotate that text annotations always carry (§12.5.6.4).
func EffectiveAnnotFlags(a Annotation) Flags {
	flags := a.GetCommon().Flags
	if _, ok := a.(*Text); ok {
		flags |= FlagNoZoom | FlagNoRotate
	}
	return flags
}

// AnnotInteractionHidden reports whether flags hide the annotation from
// interactive display.  NoView annotations with ToggleNoView stay eligible:
// NoView inverts on hover (§12.5.3).
func AnnotInteractionHidden(flags Flags) bool {
	if flags&FlagHidden != 0 {
		return true
	}
	return flags&FlagNoView != 0 && flags&FlagToggleNoView == 0
}

// AnnotSuppressed reports whether an annotation should not be shown, given the
// visibility context.  forPrint selects the print flag rule (Print set,
// Hidden clear); hover selects the interactive flag rule used for rollover
// renders; otherwise the default rule (neither Hidden nor NoView) applies.  An
// annotation is also suppressed when its optional-content entry is off, or
// when hideMarkup is set and it is a markup annotation (§12.5.6.2).  ocState
// may be nil, in which case the optional-content check is skipped.
func AnnotSuppressed(a Annotation, forPrint, hover, hideMarkup bool, ocState *oc.GroupStates) bool {
	c := a.GetCommon()

	// annotation visibility flags
	switch {
	case hover:
		if AnnotInteractionHidden(c.Flags) {
			return true
		}
	case forPrint:
		if c.Flags&FlagHidden != 0 {
			return true
		}
		if c.Flags&FlagPrint == 0 {
			return true
		}
	default:
		if c.Flags&(FlagHidden|FlagNoView) != 0 {
			return true
		}
	}

	// check annotation OC entry
	if ocState != nil && c.OptionalContent != nil {
		if !c.OptionalContent.IsVisible(ocState) {
			return true
		}
	}

	// optionally hide markup annotations (§12.5.6.2)
	if hideMarkup {
		if _, isMarkup := a.(MarkupAnnotation); isMarkup {
			return true
		}
	}

	return false
}
