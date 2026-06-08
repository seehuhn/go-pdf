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

// Command widget writes a PDF demonstrating the fallback appearance streams
// generated for AcroForm widget annotations. Each row shows the same field
// twice: on the left without an appearance stream (so the reader generates its
// own), and on the right with the appearance stream Quire generates. Opening
// the file in different readers allows the two to be compared.
package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/optional"
)

const (
	labelX        = 50.0
	leftColStart  = 250.0
	rightColStart = 410.0

	startY    = 800.0
	rowGap    = 12.0
	defaultDA = "/Helv 0 Tf 0.102 0.094 0.078 rg"
)

var (
	paper = color.DeviceRGB{0.992, 0.988, 0.973} // --paper
	ink   = color.DeviceRGB{0.102, 0.094, 0.078} // --ink
)

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

type writer struct {
	page   *document.Page
	style  *fallback.Style
	form   *annotation.InteractiveForm
	label  font.Layouter
	yPos   float64
	nextID int
}

// demo describes one widget appearance to show.
type demo struct {
	label    string
	width    float64
	height   float64
	ft       pdf.Name
	ff       annotation.FieldFlags
	mkCA     string // ZapfDingbats on-glyph for toggles, caption for push buttons
	value    string
	onName   pdf.Name
	opts     []string
	sel      []int
	maxLen   int
	rot      int
	style    pdf.Name // border style override (S/D/B/I/U)
	noChrome bool
}

func createDocument(filename string) error {
	paperSize := document.A4
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage(filename, paperSize, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	H := font.Must(standard.Helvetica.New())
	B := font.Must(standard.HelveticaBold.New())
	style := fallback.NewStyle()
	style.Version = pdf.V1_7

	wr := &writer{
		page:  page,
		style: style,
		form: &annotation.InteractiveForm{
			DefaultResources:  &content.Resources{Font: map[pdf.Name]font.Instance{"Helv": H}},
			DefaultAppearance: defaultDA,
		},
		label: H,
		yPos:  startY,
	}

	page.TextBegin()
	page.TextSetFont(B, 11)
	page.TextSetMatrix(matrix.Translate(leftColStart, wr.yPos))
	page.TextShow("Your PDF viewer")
	page.TextSetMatrix(matrix.Translate(rightColStart, wr.yPos))
	page.TextShow("Quire appearance stream")
	page.TextEnd()
	wr.yPos -= 22

	demos := []demo{
		{label: "Push button", width: 120, height: 26, ft: "Btn", ff: annotation.FieldPushbutton, mkCA: "Submit"},
		{label: "Check box (check)", width: 16, height: 16, ft: "Btn", mkCA: "4", onName: "Yes", value: "Yes"},
		{label: "Check box (cross)", width: 16, height: 16, ft: "Btn", mkCA: "8", onName: "Yes", value: "Yes"},
		{label: "Check box (star)", width: 16, height: 16, ft: "Btn", mkCA: "H", onName: "Yes", value: "Yes"},
		{label: "Radio button", width: 16, height: 16, ft: "Btn", ff: annotation.FieldRadio, mkCA: "l", onName: "A", value: "A"},
		{label: "Text, single line", width: 150, height: 20, ft: "Tx", value: "Ada Lovelace"},
		{label: "Text, right aligned", width: 150, height: 20, ft: "Tx", value: "1843", style: "S"},
		{label: "Text, multiline", width: 150, height: 52, ft: "Tx", ff: annotation.FieldMultiline, value: "Notes on the analytical engine and its capacity for computation."},
		{label: "Text, comb", width: 150, height: 22, ft: "Tx", ff: annotation.FieldComb, value: "AB1234", maxLen: 6},
		{label: "Text, password", width: 150, height: 20, ft: "Tx", ff: annotation.FieldPassword, value: "secret"},
		{label: "List box", width: 150, height: 72, ft: "Ch", opts: []string{"Times New Roman", "Helvetica", "Source Serif 4", "JetBrains Mono", "Palatino"}, sel: []int{2}},
		{label: "Combo box", width: 150, height: 22, ft: "Ch", ff: annotation.FieldCombo, opts: []string{"Helvetica"}, value: "Helvetica"},
		{label: "Signature (unsigned)", width: 150, height: 44, ft: "Sig"},
		{label: "Border: dashed", width: 120, height: 22, ft: "Tx", style: "D"},
		{label: "Border: beveled", width: 120, height: 22, ft: "Tx", style: "B"},
		{label: "Border: inset", width: 120, height: 22, ft: "Tx", style: "I"},
		{label: "Border: underline", width: 120, height: 22, ft: "Tx", style: "U"},
		{label: "Rotation: 90", width: 80, height: 26, ft: "Tx", value: "Aa", rot: 90},
		{label: "Rotation: 180", width: 80, height: 26, ft: "Tx", value: "Aa", rot: 180},
		{label: "Rotation: 270", width: 80, height: 26, ft: "Tx", value: "Aa", rot: 270},
	}

	for _, d := range demos {
		if err := wr.addRow(d); err != nil {
			return err
		}
	}

	formRef, err := page.RM.Store(wr.form)
	if err != nil {
		return err
	}
	page.Out.GetMeta().Catalog.AcroForm = formRef

	return page.Close()
}

func (wr *writer) addRow(d demo) error {
	rowH := d.height
	if rowH < 16 {
		rowH = 16
	}
	top := wr.yPos

	// label
	wr.page.TextBegin()
	wr.page.TextSetFont(wr.label, 9)
	wr.page.TextSetMatrix(matrix.Translate(labelX, top-rowH/2-3))
	wr.page.TextShow(d.label)
	wr.page.TextEnd()

	if err := wr.addField(d, leftColStart, top-rowH, false); err != nil {
		return err
	}
	if err := wr.addField(d, rightColStart, top-rowH, true); err != nil {
		return err
	}

	wr.yPos -= rowH + rowGap
	return nil
}

func (wr *writer) addField(d demo, x, y float64, genAP bool) error {
	rect := pdf.Rectangle{LLx: x, LLy: y, URx: x + d.width, URy: y + d.height}

	mk := &appearance.Characteristics{Rotation: d.rot}
	if !d.noChrome {
		mk.BackgroundColor = paper
		mk.BorderColor = ink
	}
	if d.mkCA != "" {
		mk.Caption = d.mkCA
	}

	bs := &annotation.BorderStyle{Width: 1, SingleUse: true}
	if d.style != "" {
		bs.Style = d.style
	}
	if d.style == "D" {
		bs.DashArray = []float64{3, 2}
	}

	wr.nextID++
	f := wr.makeField(d)

	// the widget is a child of the field and an annotation on the page; a
	// terminal field with this single widget is written as one merged object,
	// shared between the form's field list and the page's annotation list. The
	// form is stored (see createDocument) before the page is closed, so the
	// merge is in place when the page writes the widget.
	w := f.GetFieldCommon().AddWidget(rect)
	w.Common.Flags = annotation.FlagPrint
	w.MK = mk
	w.BorderStyle = bs

	isToggle := d.ft == "Btn" && d.ff&annotation.FieldPushbutton == 0
	if genAP {
		// the appearance generator also selects a toggle's on appearance
		if err := addAppearances(wr.style, f); err != nil {
			return err
		}
	} else if isToggle {
		w.AppearanceState = d.onName // show the on appearance
	}

	wr.page.Page.Annots = append(wr.page.Page.Annots, w)
	return nil
}

// makeField builds the typed form field for d as a root of wr.form. Its widget
// is added separately by the caller.
func (wr *writer) makeField(d demo) annotation.Field {
	name := fmt.Sprintf("f%d", wr.nextID)

	align := pdf.TextAlignLeft
	if d.label == "Text, right aligned" {
		align = pdf.TextAlignRight
	}

	var f annotation.Field
	switch d.ft {
	case "Btn":
		// a push button retains no value; check boxes and radio buttons carry
		// the on-state name in V, and radio buttons name their widgets in Opt
		btn := wr.form.NewButtonField(name)
		if d.ff&annotation.FieldPushbutton == 0 {
			btn.V = pdf.Name(d.value)
			if d.ff&annotation.FieldRadio != 0 && d.onName != "" {
				btn.Opt = []string{string(d.onName)}
			}
		}
		f = btn

	case "Tx":
		tx := wr.form.NewTextField(name)
		tx.DefaultAppearance = defaultDA
		tx.Align = align
		tx.MaxLen = d.maxLen
		if d.value != "" {
			tx.V = pdf.TextString(d.value)
		}
		f = tx

	case "Ch":
		ch := wr.form.NewChoiceField(name)
		ch.DefaultAppearance = defaultDA
		ch.Align = align
		ch.Selected = d.sel
		for _, o := range d.opts {
			ch.Opt = append(ch.Opt, annotation.ChoiceOption{Display: o, Export: o})
		}
		if d.value != "" {
			ch.V = pdf.TextString(d.value)
		}
		f = ch

	default: // "Sig"
		f = wr.form.NewSignatureField(name)
	}

	if d.ff != 0 {
		f.GetFieldCommon().Ff = optional.NewUInt(uint(d.ff))
	}
	return f
}

// addAppearances generates fallback appearance streams for every widget
// annotation in the field tree rooted at f, storing them in each widget's
// appearance dictionary. The field type, flags, value, and variable-text
// attributes determine what each widget draws.
func addAppearances(style *fallback.Style, f annotation.Field) error {
	params := widgetParams(f)
	on := buttonValue(f)

	widgetIndex := 0
	for _, kid := range f.GetFieldCommon().Kids {
		switch k := kid.(type) {
		case annotation.Field:
			if err := addAppearances(style, k); err != nil {
				return err
			}
		case *annotation.Widget:
			if err := addWidgetAppearance(style, params, on, f, k, widgetIndex); err != nil {
				return err
			}
			widgetIndex++
		}
	}
	return nil
}

func addWidgetAppearance(style *fallback.Style, params fallback.WidgetField, on pdf.Name, f annotation.Field, w *annotation.Widget, index int) error {
	p := params
	p.OnState = onStateFor(f, index)
	if err := style.AddWidgetAppearance(w, &p); err != nil {
		return err
	}
	// select the on appearance when this widget's on-state is the field value
	if p.OnState != "" && on == p.OnState {
		w.AppearanceState = p.OnState
	}
	return nil
}

// widgetParams builds the field context shared by all of a field's widgets.
func widgetParams(f annotation.Field) fallback.WidgetField {
	p := fallback.WidgetField{
		FieldType: annotation.ResolvedFT(f),
		Flags:     uint32(annotation.ResolvedFf(f)),
	}
	if vt, ok := f.(interface {
		GetVariableText() *annotation.VariableText
	}); ok {
		v := vt.GetVariableText()
		p.DefaultAppearance = v.DefaultAppearance
		p.Align = v.Align
	}
	switch x := f.(type) {
	case *annotation.FieldTx:
		p.Value = asText(annotation.ResolvedV(f))
		p.MaxLen = x.MaxLen
	case *annotation.FieldChoice:
		p.Options = make([]string, len(x.Opt))
		for i, o := range x.Opt {
			p.Options[i] = o.Display
		}
		p.Selected = x.Selected
		p.TopIndex = x.TopIndex
		p.Value = choiceDisplay(x)
	}
	return p
}

// onStateFor returns the on-state name of the index-th widget of a check box or
// radio button field, or the empty string for other field types.
func onStateFor(f annotation.Field, index int) pdf.Name {
	if annotation.ResolvedFT(f) != "Btn" || annotation.ResolvedFf(f)&annotation.FieldPushbutton != 0 {
		return ""
	}
	if x, ok := f.(*annotation.FieldBtn); ok {
		if index < len(x.Opt) {
			return pdf.Name(x.Opt[index])
		}
		// a single check box may name its on-state via the field value
		if x.Variant() == annotation.ButtonCheckbox && x.V != "" && x.V != "Off" {
			return x.V
		}
	}
	return "On"
}

// buttonValue returns a check box or radio button field's value name. A push
// button retains no value, so its V is empty.
func buttonValue(f annotation.Field) pdf.Name {
	if x, ok := f.(*annotation.FieldBtn); ok {
		return x.V
	}
	return ""
}

// choiceDisplay returns the display text of a choice field's current selection.
func choiceDisplay(x *annotation.FieldChoice) string {
	if len(x.Selected) > 0 {
		i := x.Selected[0]
		if i >= 0 && i < len(x.Opt) {
			return x.Opt[i].Display
		}
	}
	return asText(x.V)
}

// asText renders a stored field value as display text.
func asText(obj pdf.Object) string {
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
