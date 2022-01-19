package type1

import "seehuhn.de/go/pdf"

type FontInfo struct {
	FontName pdf.Name

	// Version is the version number of the font program.
	Version string

	// Notice is a trademark or copyright notice, if applicable.
	Notice string

	Copyright string

	// FullName is a unique, human-readable name for an individual font.
	FullName string

	// FamilyName is a human-readable name for a group of fonts that are
	// stylistic variants of a single design.  All fonts that are members of
	// such a group should have exactly the same FamilyName value.
	FamilyName string

	// A human-readable name for the weight, or "boldness," of a font.
	Weight string

	// ItalicAngle is the angle, in degrees counterclockwise from the vertical,
	// of the dominant vertical strokes of the font.
	ItalicAngle float64

	// IsFixedPitch is a flag indicating whether the font is a fixed-pitch
	// (monospaced) font.
	IsFixedPitch bool

	// UnderlinePosition is the recommended distance from the baseline for
	// positioning underlining strokes. This number is the y coordinate (in the
	// glyph coordinate system) of the center of the stroke.
	UnderlinePosition float64

	// UnderlineThickness is the recommended stroke width for underlining, in
	// units of the glyph coordinate system.
	UnderlineThickness float64

	PaintType  int32
	FontMatrix []float64

	Private []*PrivateDict
}

type PrivateDict struct {
	// BlueValues is an array containing an even number of integers.
	// The first integer in each pair is less than or equal to the second integer.
	// The first pair is the baseline overshoot position and the baseline.
	// All subsequent pairs describe alignment zones for the tops of character features.
	BlueValues []int32

	OtherBlues []int32

	BlueScale float64

	BlueShift int32

	BlueFuzz int32

	// StdHW is the dominant width of horizontal stems for glyphs in the font.
	StdHW float64

	// StdVW the dominant width of vertical stems.
	// Typically, this will be the width of straight stems in lower case letters.
	StdVW float64

	ForceBold bool
}
