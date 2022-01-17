package type1

import "seehuhn.de/go/pdf"

type FontDict struct {
	Info    *FontInfo
	Private *PrivateDict

	FontName   pdf.Name
	PaintType  int32
	FontMatrix []float64
	Encoding   map[byte]string
}

// FontInfo holds information about a font.
type FontInfo struct {
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
}

type PrivateDict struct {
	BlueValues    []float64
	OtherBlues    []float64
	BlueScale     float64
	BlueShift     int
	BlueFuzz      int
	StdHW         float64
	StdVW         float64
	ForceBold     bool
	LanguageGroup int
}
