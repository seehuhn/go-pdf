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
	"testing"

	"seehuhn.de/go/pdf"
)

// allFlags is one past the highest defined annotation flag, so ranging over it
// covers every combination the flags word can take.
const allFlags = FlagLockedContents << 1

// TestInteractionHiddenImpliesDisplayHidden checks that the hover rule is never
// stricter than the screen rule: ToggleNoView may make a NoView annotation
// visible on hover, but no flag combination hides one on hover while showing it
// on screen.
func TestInteractionHiddenImpliesDisplayHidden(t *testing.T) {
	for f := range allFlags {
		if f.InteractionHidden() && !f.DisplayHidden() {
			t.Errorf("%s: hidden on hover but shown on screen", f)
		}
	}
}

// TestHiddenSuppressesEverywhere checks that Hidden overrides every other flag
// in every context: an annotation carrying it is neither displayed, nor shown
// on hover, nor printed, whatever else is set (§12.5.3).
func TestHiddenSuppressesEverywhere(t *testing.T) {
	for f := range allFlags {
		if f&FlagHidden == 0 {
			continue
		}
		a := &Link{Common: Common{Flags: f}}
		for _, ctx := range visibilityContexts {
			if !Suppressed(a, ctx.forPrint, ctx.hover, false, nil) {
				t.Errorf("%s: not suppressed %s", f, ctx.name)
			}
		}
	}
}

// TestNoViewPrints checks the one case that forced the screen and hover rules
// apart: NoView keeps an annotation off the screen but leaves printing to the
// Print flag (§12.5.3), and ToggleNoView buys back hover visibility only.
func TestNoViewPrints(t *testing.T) {
	cases := []struct {
		name                          string
		flags                         Flags
		display, interact, printedOut bool
	}{
		{
			name:       "NoView and Print",
			flags:      FlagNoView | FlagPrint,
			printedOut: true,
		},
		{
			name:       "NoView, Print and ToggleNoView",
			flags:      FlagNoView | FlagPrint | FlagToggleNoView,
			interact:   true,
			printedOut: true,
		},
		{
			// ToggleNoView inverts NoView and has nothing to invert here
			name:     "ToggleNoView alone",
			flags:    FlagToggleNoView,
			display:  true,
			interact: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &Link{Common: Common{Flags: tc.flags}}
			want := map[string]bool{
				"on screen": tc.display,
				"on hover":  tc.interact,
				"in print":  tc.printedOut,
			}
			for _, ctx := range visibilityContexts {
				shown := !Suppressed(a, ctx.forPrint, ctx.hover, false, nil)
				if shown != want[ctx.name] {
					t.Errorf("shown %s = %t, want %t", ctx.name, shown, want[ctx.name])
				}
			}
		})
	}
}

// TestInvisibleUnknownSubtype checks that Invisible hides an annotation of an
// unknown subtype in every context, printing included, and that it is ignored
// for the standard subtypes the library understands (§12.5.3).
func TestInvisibleUnknownSubtype(t *testing.T) {
	common := Common{Flags: FlagInvisible | FlagPrint}

	unknown := &Custom{Common: common, Type: "Fictional"}
	for _, ctx := range visibilityContexts {
		if !Suppressed(unknown, ctx.forPrint, ctx.hover, false, nil) {
			t.Errorf("unknown subtype not suppressed %s", ctx.name)
		}
	}

	known := &Link{Common: common}
	for _, ctx := range visibilityContexts {
		if Suppressed(known, ctx.forPrint, ctx.hover, false, nil) {
			t.Errorf("standard subtype suppressed %s", ctx.name)
		}
	}
}

// TestNeverShownImpliesSuppressed checks that [NeverShown] keeps the promise
// its name makes: over every flag combination, for an annotation of a standard
// subtype and one of an unknown subtype, an annotation it accepts is suppressed
// in all three contexts.  The two functions derive the same rules separately,
// and this is what stops them drifting apart.
func TestNeverShownImpliesSuppressed(t *testing.T) {
	for f := range allFlags {
		common := Common{Flags: f}
		for _, a := range []Annotation{
			&Link{Common: common},
			&Custom{Common: common, Type: "Fictional"},
		} {
			if !NeverShown(a) {
				continue
			}
			for _, ctx := range visibilityContexts {
				if !Suppressed(a, ctx.forPrint, ctx.hover, false, nil) {
					t.Errorf("%T %s: never shown, yet shown %s", a, f, ctx.name)
				}
			}
		}
	}
}

// TestNeverShownIsExhaustive checks the converse, so that [NeverShown] stays
// worth calling: an annotation suppressed in all three contexts by its flags
// alone is one it accepts.
func TestNeverShownIsExhaustive(t *testing.T) {
	for f := range allFlags {
		common := Common{Flags: f}
		for _, a := range []Annotation{
			&Link{Common: common},
			&Custom{Common: common, Type: "Fictional"},
		} {
			always := true
			for _, ctx := range visibilityContexts {
				if !Suppressed(a, ctx.forPrint, ctx.hover, false, nil) {
					always = false
				}
			}
			if always && !NeverShown(a) {
				t.Errorf("%T %s: suppressed everywhere, yet not never-shown", a, f)
			}
		}
	}
}

// visibilityContexts lists the three flag rules [Suppressed] selects between.
var visibilityContexts = []struct {
	name            string
	forPrint, hover bool
}{
	{name: "on screen"},
	{name: "on hover", hover: true},
	{name: "in print", forPrint: true},
}

// TestIsReply checks which relationships mark an annotation as a reply that a
// processor shall not display on its own (§12.5.6.2, table 172).  "R" and an
// absent RT both do, "Group" does not, and neither does an annotation with no
// IRT entry at all.
func TestIsReply(t *testing.T) {
	const parent pdf.Reference = 42

	cases := []struct {
		name string
		a    Annotation
		want bool
	}{
		{"plain markup annotation", &Text{}, false},
		{"reply, RT absent", &Text{Markup: Markup{InReplyTo: parent}}, true},
		{"reply, RT=R", &Text{Markup: Markup{InReplyTo: parent, RT: "R"}}, true},
		{"group member", &Text{Markup: Markup{InReplyTo: parent, RT: "Group"}}, false},
		{"RT without IRT", &Text{Markup: Markup{RT: "R"}}, false},
		{"non-markup annotation", &Link{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsReply(tc.a); got != tc.want {
				t.Errorf("IsReply = %v, want %v", got, tc.want)
			}
		})
	}
}
