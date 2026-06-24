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

package fallback

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/text"
	"seehuhn.de/go/postscript/type1/names"
)

// widgetField is the form-field context that a widget annotation's appearance
// depends on but that the widget does not itself hold. It is resolved from the
// widget's form field (w.Field), with flag and value inheritance from ancestor
// fields already applied.
type widgetField struct {
	// FieldType is the field type: "Btn", "Tx", "Ch", or "Sig".
	FieldType pdf.Name

	// Flags is the field's effective flag bit set (the /Ff value).
	Flags acroform.FieldFlags

	// Value is the field's value rendered as text: the contents of a text
	// field, or the selected display string of a choice field.
	Value string

	// OnState is the on-state name of a check box or radio button widget (the
	// key under which its "on" appearance is stored). An empty value defaults
	// to "On".
	OnState pdf.Name

	// DefaultAppearance is the field's effective /DA string. It sets the font
	// size and colour of rendered text; a size of zero requests auto-sizing.
	DefaultAppearance string

	// Align is the text justification (the /Q value).
	Align pdf.TextAlign

	// Options holds the display strings of a choice field's items.
	Options []string

	// Selected holds the indices into Options of a choice field's selected
	// items.
	Selected []int

	// TopIndex is the index into Options of the first visible item of a
	// scrollable list box.
	TopIndex int

	// MaxLen is the comb-cell count of a text field with the comb flag set.
	MaxLen int
}

// resolveWidgetField gathers the form-field context for w from its form field
// (w.Field), or returns nil if the widget has no associated field.
func resolveWidgetField(w *annotation.Widget) *widgetField {
	f := w.Field
	if f == nil {
		return nil
	}
	p := &widgetField{
		FieldType: f.FieldType(),
		Flags:     f.GetCommon().Flags,
		OnState:   widgetOnState(f, widgetIndex(w)),
	}
	if vt, ok := f.(acroform.VariableTextField); ok {
		v := vt.GetVariableText()
		p.DefaultAppearance = v.DefaultAppearance
		p.Align = v.Align
	}
	switch x := f.(type) {
	case *acroform.TextField:
		p.Value = valueText(x.V)
		p.MaxLen = x.MaxLen
	case *acroform.ChoiceField:
		p.Options = make([]string, len(x.Opt))
		for i, o := range x.Opt {
			p.Options[i] = o.Display
		}
		p.Selected = x.Selected
		p.TopIndex = x.TopIndex
		p.Value = choiceValue(x)
	}
	return p
}

// widgetIndex returns the position of w among the widgets of its field. Radio
// buttons name each widget's on-state by this index.
func widgetIndex(w *annotation.Widget) int {
	if w.Field == nil {
		return 0
	}
	for i, kw := range w.Field.GetCommon().Widgets {
		if kw == acroform.Widget(w) {
			return i
		}
	}
	return 0
}

// widgetOnState returns the on-state name of the index-th widget of a check box
// or radio button field, or the empty string for other field types.
func widgetOnState(f acroform.Field, index int) pdf.Name {
	ff := f.GetCommon().Flags
	if f.FieldType() != "Btn" || ff&acroform.FieldPushbutton != 0 {
		return ""
	}
	if btn, ok := f.(*acroform.ButtonField); ok && index < len(btn.Opt) {
		return pdf.Name(btn.Opt[index])
	}
	// a single check box may name its on-state via the field value
	if ff&acroform.FieldRadio == 0 {
		if v := buttonValue(f); v != "" && v != "Off" {
			return v
		}
	}
	return "On"
}

// buttonValue returns a check box or radio button field's value name.
func buttonValue(f acroform.Field) pdf.Name {
	if b, ok := f.(*acroform.ButtonField); ok {
		return b.V
	}
	return ""
}

// choiceValue returns the display text of a choice field's current selection.
func choiceValue(x *acroform.ChoiceField) string {
	if len(x.Selected) > 0 {
		i := x.Selected[0]
		if i >= 0 && i < len(x.Opt) {
			return x.Opt[i].Display
		}
	}
	return valueText(x.V)
}

// valueText renders a stored field value as display text.
func valueText(obj pdf.Object) string {
	switch v := obj.(type) {
	case pdf.String:
		return string(v.AsTextString())
	case pdf.TextString:
		return string(v)
	case pdf.Name:
		return string(v)
	default:
		return ""
	}
}

// addWidgetAppearance generates the appearance stream(s) for a form-field widget
// annotation and stores them in the widget's appearance dictionary. The field
// context is resolved from the widget's form field (w.Field); a widget with no
// field draws only the Style chrome. It is the dispatch target of
// [Style.AddAppearance] for [annotation.Widget].
//
// Check boxes and radio buttons receive two appearances, keyed by the on-state
// name and "Off"; all other field types receive a single normal appearance.
func (s *Style) addWidgetAppearance(w *annotation.Widget) error {
	fld := resolveWidgetField(w)
	if fld == nil {
		chrome, err := s.drawChromeField(w)
		if err != nil {
			return err
		}
		w.Appearance = &appearance.Dict{SingleUse: true, Normal: chrome}
		w.AppearanceState = ""
		return nil
	}

	if fld.FieldType == "Btn" && fld.Flags&acroform.FieldPushbutton == 0 {
		// check box or radio button: an on and an off appearance
		on, err := s.drawToggle(w, fld, true)
		if err != nil {
			return err
		}
		off, err := s.drawToggle(w, fld, false)
		if err != nil {
			return err
		}
		onState := fld.OnState
		if onState == "" {
			onState = "On"
		}
		w.Appearance = &appearance.Dict{
			SingleUse: true,
			NormalMap: map[pdf.Name]*form.Form{onState: on, "Off": off},
		}
		// select the current appearance from the field value
		if w.AppearanceState == "" {
			if buttonValue(w.Field) == onState {
				w.AppearanceState = onState
			} else {
				w.AppearanceState = "Off"
			}
		}
		return nil
	}

	var normal *form.Form
	var err error
	switch fld.FieldType {
	case "Btn":
		normal, err = s.drawPushButton(w)
	case "Tx":
		normal, err = s.drawTextField(w, fld)
	case "Ch":
		normal, err = s.drawChoiceField(w, fld)
	default: // Sig and anything else: chrome only
		normal, err = s.drawChromeField(w)
	}
	if err != nil {
		return err
	}

	w.Appearance = &appearance.Dict{SingleUse: true, Normal: normal}
	w.AppearanceState = ""
	return nil
}

// fieldContext sets up a content builder for a widget's appearance and returns
// the local drawing box (always rooted at the origin), the placement matrix
// that maps it onto the widget rectangle with the MK rotation applied, and the
// builder.
func (s *Style) fieldContext(w *annotation.Widget) (b *builder.Builder, width, height float64, m matrix.Matrix) {
	rot := 0
	if w.Style != nil {
		rot = ((w.Style.Rotation % 360) + 360) % 360
	}
	rw := pdf.Round(w.Rect.Dx(), 2)
	rh := pdf.Round(w.Rect.Dy(), 2)
	llx, lly := pdf.Round(w.Rect.LLx, 2), pdf.Round(w.Rect.LLy, 2)

	switch rot {
	case 90:
		width, height = rh, rw
		m = matrix.Matrix{0, 1, -1, 0, llx + rw, lly}
	case 180:
		width, height = rw, rh
		m = matrix.Matrix{-1, 0, 0, -1, llx + rw, lly + rh}
	case 270:
		width, height = rh, rw
		m = matrix.Matrix{0, -1, 1, 0, llx, lly + rh}
	default:
		width, height = rw, rh
		m = matrix.Matrix{1, 0, 0, 1, llx, lly}
	}

	b = builder.New(content.Form, nil, s.version)
	b.SetExtGState(s.reset)
	return b, width, height, m
}

func (s *Style) finishForm(b *builder.Builder, width, height float64, m matrix.Matrix) (*form.Form, error) {
	ops, err := b.Harvest()
	if err != nil {
		return nil, err
	}
	return &form.Form{
		Content: ops,
		Res:     b.Resources,
		BBox:    pdf.Rectangle{LLx: 0, LLy: 0, URx: width, URy: height},
		Matrix:  m,
	}, nil
}

// drawChromeField draws background and border only.
func (s *Style) drawChromeField(w *annotation.Widget) (*form.Form, error) {
	b, width, height, m := s.fieldContext(w)
	drawChrome(b, width, height, w)
	return s.finishForm(b, width, height, m)
}

func borderWidth(w *annotation.Widget) float64 {
	if w.BorderStyle != nil {
		return w.BorderStyle.Width
	}
	if w.Border != nil {
		return w.Border.Width
	}
	return 1
}

func borderStyleName(w *annotation.Widget) pdf.Name {
	if w.BorderStyle != nil && w.BorderStyle.Style != "" {
		return w.BorderStyle.Style
	}
	return "S"
}

// drawChrome fills the background and strokes the border of a rectangular field
// according to the widget's Style and border-style entries.
func drawChrome(b *builder.Builder, width, height float64, w *annotation.Widget) {
	mk := w.Style
	if mk != nil && mk.BackgroundColor != nil {
		b.SetFillColor(mk.BackgroundColor)
		b.Rectangle(0, 0, width, height)
		b.Fill()
	}

	lw := borderWidth(w)
	if mk == nil || mk.BorderColor == nil || lw <= 0 {
		return
	}

	style := borderStyleName(w)
	switch style {
	case "U": // underline: a single rule along the bottom edge
		b.SetLineWidth(lw)
		b.SetStrokeColor(mk.BorderColor)
		b.MoveTo(0, lw/2)
		b.LineTo(width, lw/2)
		b.Stroke()
	case "B", "I": // beveled (raised) or inset (sunken)
		// a border-colour outline with the 3-D bands inside it; the band
		// shades are fixed (white highlight, ink-4 shadow), light from the
		// top left for a raised border, from the bottom right for a sunken one
		topLeft, bottomRight := quireWhite, quireInk4
		if style == "I" {
			topLeft, bottomRight = bottomRight, topLeft
		}
		inner := pdf.Rectangle{LLx: lw, LLy: lw, URx: width - lw, URy: height - lw}
		drawBevelBands(b, inner, lw, topLeft, bottomRight)
		b.SetLineWidth(lw)
		b.SetStrokeColor(mk.BorderColor)
		b.Rectangle(lw/2, lw/2, width-lw, height-lw)
		b.Stroke()
	default: // "S" solid, "D" dashed
		b.SetLineWidth(lw)
		b.SetStrokeColor(mk.BorderColor)
		if style == "D" {
			dash := []float64{3}
			if w.BorderStyle != nil && len(w.BorderStyle.DashArray) > 0 {
				dash = w.BorderStyle.DashArray
			}
			b.SetLineDash(dash, 0)
		}
		b.Rectangle(lw/2, lw/2, width-lw, height-lw)
		b.Stroke()
	}
}

// drawToggle draws one appearance of a check box or radio button: the on-glyph
// when on is true, chrome only otherwise. Radio buttons use a circular border.
func (s *Style) drawToggle(w *annotation.Widget, fld *widgetField, on bool) (*form.Form, error) {
	b, width, height, m := s.fieldContext(w)
	isRadio := fld.Flags&acroform.FieldRadio != 0

	if isRadio {
		drawCircleChrome(b, width, height, w)
	} else {
		drawChrome(b, width, height, w)
	}

	if on {
		glyph := ""
		if w.Style != nil {
			glyph = w.Style.Caption
		}
		if glyph == "" {
			if isRadio {
				glyph = "l" // ZapfDingbats filled circle
			} else {
				glyph = "4" // ZapfDingbats check mark
			}
		}
		s.drawDingbat(b, width, height, glyph)
	}

	return s.finishForm(b, width, height, m)
}

// drawCircleChrome fills and strokes a circular field background and border.
func drawCircleChrome(b *builder.Builder, width, height float64, w *annotation.Widget) {
	mk := w.Style
	cx, cy := width/2, height/2
	outer := min(width, height) / 2
	lw := borderWidth(w)

	if mk != nil && mk.BackgroundColor != nil {
		b.SetFillColor(mk.BackgroundColor)
		b.Circle(cx, cy, outer)
		b.Fill()
	}
	if mk != nil && mk.BorderColor != nil && lw > 0 {
		b.SetLineWidth(lw)
		b.SetStrokeColor(mk.BorderColor)
		b.Circle(cx, cy, outer-lw/2)
		b.Stroke()
	}
}

// drawDingbat centres a ZapfDingbats glyph in the field box. The marker is named
// by its code in the ZapfDingbats encoding (the MK.CA convention, e.g. "4" for a
// check mark); it is translated to the glyph's Unicode value so that the font's
// character map selects the intended glyph.
func (s *Style) drawDingbat(b *builder.Builder, width, height float64, marker string) {
	glyph := dingbatText(marker)
	if glyph == "" {
		return
	}
	size := min(width, height) * 0.62
	b.TextBegin()
	b.TextSetFont(s.dingbatsFont, size)
	b.SetFillColor(quireInk)
	b.TextFirstLine(0, height/2-size*0.33)
	b.TextShowAligned(glyph, width, 0.5)
	b.TextEnd()
}

// dingbatText translates a marker given as ZapfDingbats encoding codes (the
// MK.CA convention) into the corresponding Unicode text.
func dingbatText(marker string) string {
	var sb strings.Builder
	for i := 0; i < len(marker); i++ {
		name := pdfenc.ZapfDingbats.Encoding[marker[i]]
		if name == "" || name == ".notdef" {
			continue
		}
		sb.WriteString(names.ToUnicode(name, "ZapfDingbats"))
	}
	return sb.String()
}

// drawPushButton draws a push button's chrome and centred caption.
func (s *Style) drawPushButton(w *annotation.Widget) (*form.Form, error) {
	b, width, height, m := s.fieldContext(w)
	drawChrome(b, width, height, w)

	if w.Style != nil && w.Style.Caption != "" {
		size := captionSize(height)
		b.TextBegin()
		b.TextSetFont(s.ContentFont, size)
		b.SetFillColor(quireInk)
		b.TextFirstLine(0, height/2-size*0.33)
		b.TextShowAligned(w.Style.Caption, width, 0.5)
		b.TextEnd()
	}

	return s.finishForm(b, width, height, m)
}

// drawTextField draws a text field's chrome and value.
func (s *Style) drawTextField(w *annotation.Widget, fld *widgetField) (*form.Form, error) {
	b, width, height, m := s.fieldContext(w)
	drawChrome(b, width, height, w)

	lw := borderWidth(w)
	const pad = 2.0

	switch {
	case fld.Flags&acroform.FieldPassword != 0:
		// the value is masked: one bullet per character
		s.drawSingleLine(b, width, height, lw, pad, fld, strings.Repeat("*", utf8.RuneCountInString(fld.Value)))
	case fld.Flags&acroform.FieldComb != 0 && fld.MaxLen > 0:
		s.drawComb(b, width, height, lw, fld)
	case fld.Flags&acroform.FieldMultiline != 0:
		s.drawMultiline(b, width, height, lw, pad, fld)
	case fld.Value != "":
		s.drawSingleLine(b, width, height, lw, pad, fld, fld.Value)
	}

	return s.finishForm(b, width, height, m)
}

// drawSingleLine draws text as a single, vertically centred line.
func (s *Style) drawSingleLine(b *builder.Builder, width, height, lw, pad float64, fld *widgetField, text string) {
	if text == "" {
		return
	}
	size, col := parseDA(fld.DefaultAppearance)
	if size == 0 {
		size = autoSize(height - 2*lw)
	}
	left := lw + pad
	contentWidth := width - 2*(lw+pad)

	b.PushGraphicsState()
	b.Rectangle(left, lw, contentWidth, height-2*lw)
	b.ClipNonZero()
	b.EndPath()

	b.TextBegin()
	b.TextSetFont(s.ContentFont, size)
	b.SetFillColor(col)
	b.TextFirstLine(left, height/2-size*0.33)
	b.TextShowAligned(text, contentWidth, quadFraction(fld.Align))
	b.TextEnd()

	b.PopGraphicsState()
}

// drawMultiline draws word-wrapped, top-aligned field text.
func (s *Style) drawMultiline(b *builder.Builder, width, height, lw, pad float64, fld *widgetField) {
	if fld.Value == "" {
		return
	}
	F := s.ContentFont
	size, col := parseDA(fld.DefaultAppearance)
	if size == 0 {
		size = 11
	}
	left := lw + pad
	contentWidth := width - 2*(lw+pad)
	lineHeight := pdf.Round(F.GetGeometry().Leading*size, 2)

	b.PushGraphicsState()
	b.Rectangle(left, lw, contentWidth, height-2*lw)
	b.ClipNonZero()
	b.EndPath()

	b.TextBegin()
	b.TextSetFont(F, size)
	b.SetFillColor(col)
	yPos := height - lw - pad - size
	lineNo := 0
	wrapper := text.Wrap(contentWidth, fld.Value)
	for line := range wrapper.Lines(F, size) {
		switch lineNo {
		case 0:
			b.TextFirstLine(left, yPos)
		case 1:
			b.TextSecondLine(0, -lineHeight)
		default:
			b.TextNextLine()
		}
		switch fld.Align {
		case pdf.TextAlignCenter:
			line.Align(contentWidth, 0.5)
		case pdf.TextAlignRight:
			line.Align(contentWidth, 1.0)
		}
		b.TextShowGlyphs(line)
		lineNo++
	}
	b.TextEnd()
	b.PopGraphicsState()
}

// drawComb lays the value out into MaxLen equal cells separated by hairline
// rules, one character per cell. A value shorter than MaxLen occupies the
// left, middle or right cells, following the field's justification.
func (s *Style) drawComb(b *builder.Builder, width, height, lw float64, fld *widgetField) {
	n := fld.MaxLen
	cellW := (width - 2*lw) / float64(n)

	b.SetLineWidth(0.6)
	b.SetStrokeColor(quireInk)
	for i := 1; i < n; i++ {
		x := pdf.Round(lw+cellW*float64(i), 2)
		b.MoveTo(x, pdf.Round(lw, 2))
		b.LineTo(x, pdf.Round(height-lw, 2))
	}
	b.Stroke()

	runes := []rune(fld.Value)
	k := min(len(runes), n)
	if k == 0 {
		return
	}
	start := 0
	switch fld.Align {
	case pdf.TextAlignCenter:
		start = (n - k) / 2
	case pdf.TextAlignRight:
		start = n - k
	}

	size, col := parseDA(fld.DefaultAppearance)
	if size == 0 {
		size = autoSize(height - 2*lw)
	}
	baseline := pdf.Round(height/2-size*0.33, 2)
	b.TextBegin()
	b.TextSetFont(s.ContentFont, size)
	b.SetFillColor(col)
	prev := 0.0
	for i := range k {
		// cell positions rounded individually, so the relative moves do
		// not accumulate rounding drift
		x := pdf.Round(lw+cellW*float64(start+i), 2)
		if i == 0 {
			b.TextFirstLine(x, baseline)
		} else {
			b.TextFirstLine(pdf.Round(x-prev, 2), 0)
		}
		prev = x
		b.TextShowAligned(string(runes[i]), cellW, 0.5)
	}
	b.TextEnd()
}

// drawChoiceField draws a list box or, when the combo flag is set, a combo box.
func (s *Style) drawChoiceField(w *annotation.Widget, fld *widgetField) (*form.Form, error) {
	b, width, height, m := s.fieldContext(w)
	drawChrome(b, width, height, w)
	lw := borderWidth(w)
	const pad = 2.0

	if fld.Flags&acroform.FieldCombo != 0 {
		s.drawCombo(b, width, height, lw, pad, fld)
	} else {
		s.drawListBox(b, width, height, lw, pad, fld)
	}

	return s.finishForm(b, width, height, m)
}

// drawCombo draws the selected value as a single line plus a divider and a
// disclosure chevron at the right edge.
func (s *Style) drawCombo(b *builder.Builder, width, height, lw, pad float64, fld *widgetField) {
	chevronW := height
	divX := width - chevronW

	b.SetLineWidth(0.6)
	b.SetStrokeColor(quireInk)
	b.MoveTo(divX, lw)
	b.LineTo(divX, height-lw)
	b.Stroke()

	cx := divX + chevronW/2
	cy := height / 2
	d := min(chevronW, height) * 0.18
	b.PushGraphicsState()
	b.SetLineWidth(1.2)
	b.SetLineCap(graphics.LineCapRound)
	b.SetLineJoin(graphics.LineJoinRound)
	b.SetStrokeColor(quireInk)
	b.MoveTo(cx-d, cy+d/2)
	b.LineTo(cx, cy-d/2)
	b.LineTo(cx+d, cy+d/2)
	b.Stroke()
	b.PopGraphicsState()

	if fld.Value != "" {
		size, col := parseDA(fld.DefaultAppearance)
		if size == 0 {
			size = autoSize(height - 2*lw)
		}
		left := lw + pad
		b.TextBegin()
		b.TextSetFont(s.ContentFont, size)
		b.SetFillColor(col)
		b.TextFirstLine(left, height/2-size*0.33)
		b.TextShowAligned(fld.Value, divX-left-pad, quadFraction(fld.Align))
		b.TextEnd()
	}
}

// drawListBox draws the option items, scrolled to TopIndex, with the selected
// rows highlighted in the Quire selection treatment (amber wash and indicator).
func (s *Style) drawListBox(b *builder.Builder, width, height, lw, pad float64, fld *widgetField) {
	size, col := parseDA(fld.DefaultAppearance)
	if size == 0 {
		size = 11
	}
	rowH := pdf.Round(s.ContentFont.GetGeometry().Leading*size, 2)
	if rowH <= 0 {
		rowH = size * 1.3
	}
	left := lw + pad

	selected := map[int]bool{}
	for _, i := range fld.Selected {
		selected[i] = true
	}

	b.PushGraphicsState()
	b.Rectangle(lw, lw, width-2*lw, height-2*lw)
	b.ClipNonZero()
	b.EndPath()

	top := height - lw
	for i := fld.TopIndex; i < len(fld.Options); i++ {
		rowTop := top - rowH*float64(i-fld.TopIndex)
		rowBottom := rowTop - rowH
		if rowTop < lw {
			break
		}
		if selected[i] {
			b.SetFillColor(quireAmber50)
			b.Rectangle(lw, rowBottom, width-2*lw, rowH)
			b.Fill()
			b.SetFillColor(quireAmber400)
			b.Rectangle(lw, rowBottom, 2, rowH)
			b.Fill()
		}
		b.TextBegin()
		b.TextSetFont(s.ContentFont, size)
		b.SetFillColor(col)
		b.TextFirstLine(left, rowBottom+(rowH-size)/2+size*0.2)
		b.TextShow(fld.Options[i])
		b.TextEnd()
	}

	b.PopGraphicsState()
}

// quadFraction maps a text alignment to the fraction used by TextShowAligned.
func quadFraction(q pdf.TextAlign) float64 {
	switch q {
	case pdf.TextAlignCenter:
		return 0.5
	case pdf.TextAlignRight:
		return 1.0
	default:
		return 0.0
	}
}

// captionSize chooses a push-button caption size that fits the field height.
func captionSize(height float64) float64 {
	return clampFloat(height*0.5, 6, 12)
}

// autoSize chooses a text size for a single line in a field of the given inner
// height, used when the DA font size is zero.
func autoSize(innerHeight float64) float64 {
	return clampFloat(innerHeight*0.72, 6, 12)
}

func clampFloat(v, lo, hi float64) float64 {
	return max(lo, min(hi, v))
}

// parseDA extracts the text size and colour from a default-appearance string.
// Unrecognised input yields a zero size (auto) and the default ink colour.
func parseDA(da string) (size float64, col color.Color) {
	col = quireInk
	fields := strings.Fields(da)
	for i, tok := range fields {
		switch tok {
		case "Tf":
			if i >= 1 {
				if v, ok := daFloat(fields[i-1]); ok && v >= 0 {
					size = v
				}
			}
		case "g":
			if i >= 1 {
				if g, ok := daFloat(fields[i-1]); ok {
					col = color.DeviceGray(g)
				}
			}
		case "rg":
			if i >= 3 {
				r, ok1 := daFloat(fields[i-3])
				g, ok2 := daFloat(fields[i-2])
				bl, ok3 := daFloat(fields[i-1])
				if ok1 && ok2 && ok3 {
					col = color.DeviceRGB{r, g, bl}
				}
			}
		case "k":
			if i >= 4 {
				c, ok1 := daFloat(fields[i-4])
				mm, ok2 := daFloat(fields[i-3])
				y, ok3 := daFloat(fields[i-2])
				kk, ok4 := daFloat(fields[i-1])
				if ok1 && ok2 && ok3 && ok4 {
					col = color.DeviceCMYK{c, mm, y, kk}
				}
			}
		}
	}
	return size, col
}

func daFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
