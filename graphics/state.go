// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package graphics

import (
	"slices"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/color"
)

// ClipPath represents a clipping region added by the W or W* operator.
type ClipPath struct {
	Path    *path.Data
	EvenOdd bool
	CTM     matrix.Matrix
}

// DefaultMiterLimit is the miter limit of a graphics state which does not set
// one.  A join whose miter is longer than this multiple of the line width is
// bevelled instead, which happens where the segments enclose an angle of less
// than about 11.5 degrees.
const DefaultMiterLimit = path.DefaultMiterLimit

// State represents the current graphics state of a PDF processor,
// within a content stream.  When reading or writing content streams,
// this is updated after each operator in the stream.
//
// Not all fields of State are required to be set at all times.
// The bitmask State.Set indicates which fields are valid/used.
type State struct {
	// CTM is the "current transformation matrix", which maps positions from
	// user coordinates to device coordinates.
	CTM matrix.Matrix

	// StrokeColor is the color used for stroking operations.
	StrokeColor color.Color

	// FillColor is the color used for filling and other non-stroking operations.
	FillColor color.Color

	// TextCharacterSpacing is extra spacing between glyphs, in unscaled text space units.
	TextCharacterSpacing float64

	// TextWordSpacing is extra spacing between words, in unscaled text space units.
	TextWordSpacing float64

	// TextHorizontalScaling is the horizontal scaling factor, where 1 means the natural width.
	TextHorizontalScaling float64

	// TextLeading is the vertical distance between baselines, in unscaled text space units.
	TextLeading float64

	// TextFont is the current font.
	TextFont font.Instance

	// TextFontSize is the size at which glyphs are rendered.
	TextFontSize float64

	// TextRenderingMode controls how glyphs are painted.
	TextRenderingMode TextRenderingMode

	// TextRise is the vertical offset for superscript/subscript, in unscaled text space units.
	TextRise float64

	// TextKnockout controls whether overlapping glyphs knock out or overprint.
	TextKnockout bool

	// TextMatrix is the text matrix, reset at the start of each text object.
	TextMatrix matrix.Matrix

	// TextLineMatrix is the text line matrix, reset at the start of each text object.
	TextLineMatrix matrix.Matrix

	// LineWidth is the thickness of stroked paths, in user space units.
	// The value must be non-negative.
	//
	// The value 0 stands for the thinnest line that can be rendered at device
	// resolution.  Obviously, the result is device-dependent.
	LineWidth float64

	// LineCap is the shape used at the ends of open stroked paths.
	LineCap LineCapStyle

	// LineJoin is the shape used at corners of stroked paths.
	LineJoin LineJoinStyle

	// MiterLimit is the maximum miter length to line width ratio for mitered
	// joins.  The value must be at least 1.  See [DefaultMiterLimit].
	MiterLimit float64

	// DashPattern specifies the lengths of alternating dashes and gaps, in user space units.
	DashPattern []float64

	// DashPhase is the distance into the dash pattern at which to start, in user space units.
	DashPhase float64

	// RenderingIntent specifies how CIE-based colors are converted to device colors.
	RenderingIntent RenderingIntent

	// StrokeAdjustment controls whether to compensate for rasterization effects
	// when stroking paths with small line widths.
	StrokeAdjustment bool

	// BlendMode is the blend mode for the transparent imaging model.
	BlendMode BlendMode

	// SoftMask specifies mask shape or opacity values for transparency.
	SoftMask SoftClip

	// SoftMaskCTM is the CTM in effect when the soft mask was established.
	// It is meaningful only while SoftMask is non-nil.
	SoftMaskCTM matrix.Matrix

	// StrokeAlpha is the constant opacity for stroking operations.
	// The value must be in the range 0 (transparent) to 1 (opaque).
	StrokeAlpha float64

	// FillAlpha is the constant opacity for non-stroking operations.
	// The value must be in the range 0 (transparent) to 1 (opaque).
	FillAlpha float64

	// AlphaSourceFlag specifies whether soft mask and alpha are interpreted
	// as shape values (true) or opacity values (false).
	AlphaSourceFlag bool

	// BlackPointCompensation controls the black point compensation algorithm
	// for CIE-based color conversions (PDF 2.0).
	BlackPointCompensation pdf.Name

	// OverprintStroke controls whether stroking in one colorant erases other colorants.
	OverprintStroke bool

	// OverprintFill controls whether filling in one colorant erases other colorants.
	OverprintFill bool

	// OverprintMode controls how zero values in DeviceCMYK are treated when overprinting.
	OverprintMode int

	// BlackGeneration specifies the black generation function to be used for
	// color conversion from DeviceRGB to DeviceCMYK.  The value nil represents
	// the device-dependent default function.
	BlackGeneration pdf.Function

	// UndercolorRemoval specifies the undercolor removal function to be used
	// for color conversion from DeviceRGB to DeviceCMYK.  The value nil
	// represents the device-dependent default function.
	UndercolorRemoval pdf.Function

	// TransferFunctions (deprecated in PDF 2.0) represents the transfer
	// functions for the individual color components.
	TransferFunctions TransferFunctions

	// Halftone specifies the halftone screen to be used.
	// The value nil represents the device-dependent default halftone.
	Halftone Halftone

	// HalftoneOriginX (PDF 2.0) is the X coordinate of the halftone origin.
	HalftoneOriginX float64

	// HalftoneOriginY (PDF 2.0) is the Y coordinate of the halftone origin.
	HalftoneOriginY float64

	// FlatnessTolerance specifies the precision with which curves are
	// rendered on the output device, as a maximum error in device pixels.
	// Smaller numbers give smoother curves, but also increase the amount of
	// computation needed.  The value must be in the range 0 to 100, where 0
	// selects the output device's default tolerance.
	FlatnessTolerance float64

	// SmoothnessTolerance controls the precision for rendering color
	// gradients, as a fraction of the range of each color component.
	// The value must be in the range 0 (accurate) to 1 (fast).
	SmoothnessTolerance float64

	// ClipPaths is the list of active clipping paths.
	// The effective clip region is the intersection of all entries.
	ClipPaths []ClipPath

	// Set indicates which of the parameters above are valid/used.
	Set Bits
}

// Clone returns a copy of the graphics state.
func (s *State) Clone() *State {
	clone := *s
	clone.DashPattern = slices.Clone(s.DashPattern)
	clone.ClipPaths = slices.Clone(s.ClipPaths)
	return &clone
}

// NewState returns a new graphics state with parameters initialized to
// their default values for page content streams, as defined in PDF 32000-1:2008
// Tables 51 and 52.
func NewState() State {
	return State{
		CTM: matrix.Identity,

		StrokeColor: color.Black,
		FillColor:   color.Black,

		TextCharacterSpacing:  0,
		TextWordSpacing:       0,
		TextHorizontalScaling: 1,
		TextLeading:           0,
		// no default for TextFont
		// no default for TextFontSize
		TextRenderingMode: 0,
		TextRise:          0,
		TextKnockout:      true,

		// TextMatrix and TextLineMatrix are reset at the start of each text object

		LineWidth:   1,
		LineCap:     LineCapButt,
		LineJoin:    LineJoinMiter,
		MiterLimit:  DefaultMiterLimit,
		DashPattern: []float64{},
		DashPhase:   0,

		RenderingIntent:        RelativeColorimetric,
		StrokeAdjustment:       false,
		BlendMode:              BlendMode{BlendModeNormal},
		SoftMask:               nil,
		StrokeAlpha:            1,
		FillAlpha:              1,
		AlphaSourceFlag:        false,
		BlackPointCompensation: pdf.Name("Default"),

		OverprintStroke: false,
		OverprintFill:   false,
		OverprintMode:   0,

		BlackGeneration:   nil, // default: device dependent
		UndercolorRemoval: nil, // default: device dependent
		TransferFunctions: TransferFunctions{
			Red:   nil, // default: device dependent
			Green: nil, // default: device dependent
			Blue:  nil, // default: device dependent
			Gray:  nil, // default: device dependent
		},

		Halftone: nil, // default: device dependent
		// HalftoneOriginX: 0, // default: device dependent
		// HalftoneOriginY: 0, // default: device dependent

		FlatnessTolerance: 1,
		// SmoothnessTolerance: 0, // default: device dependent

		Set: initializedStateBits,
	}
}

// initializedStateBits lists the parameters which are initialized to
// their default values in [NewState].
const initializedStateBits = StateStrokeColor | StateFillColor |
	StateTextCharacterSpacing | StateTextWordSpacing |
	StateTextHorizontalScaling | StateTextLeading | StateTextRenderingMode |
	StateTextRise | StateTextKnockout | StateLineWidth | StateLineCap |
	StateLineJoin | StateMiterLimit | StateLineDash | StateRenderingIntent |
	StateStrokeAdjustment | StateBlendMode | StateSoftMask |
	StateStrokeAlpha | StateFillAlpha | StateAlphaSourceFlag |
	StateBlackPointCompensation | StateOverprint | StateOverprintMode |
	StateFlatnessTolerance

// ApplyTo applies the graphics state parameters to the given state.
func (s *State) ApplyTo(other *State) {
	set := s.Set
	other.Set |= set

	if set&StateTextFont != 0 {
		other.TextFont = s.TextFont
		other.TextFontSize = s.TextFontSize
	}
	if set&StateTextKnockout != 0 {
		other.TextKnockout = s.TextKnockout
	}
	if set&StateLineWidth != 0 {
		other.LineWidth = s.LineWidth
	}
	if set&StateLineCap != 0 {
		other.LineCap = s.LineCap
	}
	if set&StateLineJoin != 0 {
		other.LineJoin = s.LineJoin
	}
	if set&StateMiterLimit != 0 {
		other.MiterLimit = s.MiterLimit
	}
	if set&StateLineDash != 0 {
		other.DashPattern = slices.Clone(s.DashPattern)
		other.DashPhase = s.DashPhase
	}
	if set&StateRenderingIntent != 0 {
		other.RenderingIntent = s.RenderingIntent
	}
	if set&StateStrokeAdjustment != 0 {
		other.StrokeAdjustment = s.StrokeAdjustment
	}
	if set&StateBlendMode != 0 {
		other.BlendMode = s.BlendMode
	}
	if set&StateSoftMask != 0 {
		other.SoftMask = s.SoftMask
		other.SoftMaskCTM = s.SoftMaskCTM
	}
	if set&StateStrokeAlpha != 0 {
		other.StrokeAlpha = s.StrokeAlpha
	}
	if set&StateFillAlpha != 0 {
		other.FillAlpha = s.FillAlpha
	}
	if set&StateAlphaSourceFlag != 0 {
		other.AlphaSourceFlag = s.AlphaSourceFlag
	}
	if set&StateBlackPointCompensation != 0 {
		other.BlackPointCompensation = s.BlackPointCompensation
	}
	if set&StateOverprint != 0 {
		other.OverprintStroke = s.OverprintStroke
		other.OverprintFill = s.OverprintFill
	}
	if set&StateOverprintMode != 0 {
		other.OverprintMode = s.OverprintMode
	}
	if set&StateBlackGeneration != 0 {
		other.BlackGeneration = s.BlackGeneration
	}
	if set&StateUndercolorRemoval != 0 {
		other.UndercolorRemoval = s.UndercolorRemoval
	}
	if set&StateTransferFunction != 0 {
		other.TransferFunctions = s.TransferFunctions
	}
	if set&StateHalftone != 0 {
		other.Halftone = s.Halftone
	}
	if set&StateHalftoneOrigin != 0 {
		other.HalftoneOriginX = s.HalftoneOriginX
		other.HalftoneOriginY = s.HalftoneOriginY
	}
	if set&StateFlatnessTolerance != 0 {
		other.FlatnessTolerance = s.FlatnessTolerance
	}
	if set&StateSmoothnessTolerance != 0 {
		other.SmoothnessTolerance = s.SmoothnessTolerance
	}
}

// GetTextPositionDevice returns the current text position in device coordinates.
func (s *State) GetTextPositionDevice() (float64, float64) {
	if err := s.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		panic(err)
	}
	M := s.glyphScale()
	M = M.Mul(s.TextMatrix)
	M = M.Mul(s.CTM)
	return M[4], M[5]
}

// TextRenderingMatrix returns the text rendering matrix in device coordinates.
// It is the permissive equivalent of [State.GetTextPositionDevice] (extended to
// the full 3x3 matrix): it uses field values without checking which state
// parameters have been explicitly set, so it does not panic when called from a
// content-stream reader that has not seen every text-state operator.  When the
// caller has not set TextFontSize or TextHorizontalScaling, the returned
// matrix is degenerate.
func (s *State) TextRenderingMatrix() matrix.Matrix {
	M := s.glyphScale()
	M = M.Mul(s.TextMatrix)
	return M.Mul(s.CTM)
}

// glyphScale returns the matrix [Tfs×Th 0 0 Tfs 0 Trise] which maps glyph
// space to text space.
func (s *State) glyphScale() matrix.Matrix {
	return matrix.Matrix{s.TextFontSize * s.TextHorizontalScaling, 0, 0, s.TextFontSize, 0, s.TextRise}
}

// GetTextPositionUser returns the current text position in user coordinates.
func (s *State) GetTextPositionUser() (float64, float64) {
	if err := s.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		panic(err)
	}
	M := s.glyphScale()
	M = M.Mul(s.TextMatrix)
	return M[4], M[5]
}

// GlyphMatrix returns the matrix which maps the outline of the glyph
// described by info to user space, for a glyph shown at the current text
// position.  The outline is given in text space units, relative to the
// horizontal glyph origin.
//
// In horizontal writing mode the horizontal glyph origin is placed at the
// current text position, and the result is the text rendering matrix without
// the CTM.  In vertical writing mode the current text position is the
// vertical glyph origin, so the outline is first shifted by minus the position
// vector ([font.Code.VerticalOriginX], [font.Code.VerticalOriginY]):
//
//	[1 0 0 1 -vx -vy] × [Tfs×Th 0 0 Tfs 0 Trise] × Tm
//
// Calling this without a font set gives the matrix for horizontal writing.
func (s *State) GlyphMatrix(info *font.Code) matrix.Matrix {
	M := s.glyphScale()
	if s.TextFont != nil && s.TextFont.WritingMode() == font.Vertical {
		M = matrix.Translate(-info.VerticalOriginX, -info.VerticalOriginY).Mul(M)
	}
	return M.Mul(s.TextMatrix)
}

// AdvanceTextMatrix updates [State.TextMatrix] to account for the displacement
// of a single decoded glyph.  The advance combines the font's glyph metric
// (Width for horizontal writing mode, VerticalAdvance for vertical) with the
// current font size, character spacing, word spacing (when
// [font.Code.UseWordSpacing] is set), and — in horizontal writing mode —
// horizontal scaling.
//
// Calling this without a font set is a no-op, matching the permissive
// behaviour of content-stream consumers that may not have seen a Tf
// operator yet.
func (s *State) AdvanceTextMatrix(info *font.Code) {
	if s.TextFont == nil {
		return
	}
	switch s.TextFont.WritingMode() {
	case font.Horizontal:
		advance := info.Width*s.TextFontSize + s.TextCharacterSpacing
		if info.UseWordSpacing {
			advance += s.TextWordSpacing
		}
		advance *= s.TextHorizontalScaling
		s.TextMatrix = matrix.Translate(advance, 0).Mul(s.TextMatrix)
	case font.Vertical:
		advance := info.VerticalAdvance*s.TextFontSize + s.TextCharacterSpacing
		if info.UseWordSpacing {
			advance += s.TextWordSpacing
		}
		s.TextMatrix = matrix.Translate(0, advance).Mul(s.TextMatrix)
	}
}

// ApplyTextKern applies a kerning adjustment to [State.TextMatrix].  delta is
// the raw numeric value from a TJ-array element, in thousandths of an em;
// positive values shrink the gap, negative values widen it (Table 107).
//
// Calling this without a font set is a no-op, matching the permissive
// behaviour of content-stream consumers that may not have seen a Tf
// operator yet.
func (s *State) ApplyTextKern(delta float64) {
	if delta == 0 || s.TextFont == nil {
		return
	}
	delta *= -s.TextFontSize / 1000
	switch s.TextFont.WritingMode() {
	case font.Horizontal:
		s.TextMatrix = matrix.Translate(delta*s.TextHorizontalScaling, 0).Mul(s.TextMatrix)
	case font.Vertical:
		s.TextMatrix = matrix.Translate(0, delta).Mul(s.TextMatrix)
	}
}

func (s *State) mustBeSet(bits Bits) error {
	missing := ^s.Set & bits
	if missing == 0 {
		return nil
	}
	return ErrMissing(missing)
}
