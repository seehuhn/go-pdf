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
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
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

	// options shared by the list box and combo box demos: the five font
	// families of the 14 standard PDF fonts
	fontOpts = []string{"Courier", "Helvetica", "Times", "Symbol", "ZapfDingbats"}
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
	form   *acroform.InteractiveForm
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
	ff       acroform.FieldFlags
	mkCA     string // ZapfDingbats on-glyph for toggles, caption for push buttons
	align    pdf.TextAlign
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
	style := fallback.NewStyle(pdf.V1_7)

	wr := &writer{
		page:  page,
		style: style,
		form: &acroform.InteractiveForm{
			DefaultResources: &content.Resources{Font: map[pdf.Name]font.Instance{"Helv": H}},
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
		{label: "Push button", width: 120, height: 26, ft: "Btn", ff: acroform.FieldPushbutton, mkCA: "Submit"},
		{label: "Check box (check)", width: 16, height: 16, ft: "Btn", mkCA: "4", onName: "Yes", value: "Yes"},
		{label: "Check box (cross)", width: 16, height: 16, ft: "Btn", mkCA: "8", onName: "Yes", value: "Yes"},
		{label: "Check box (star)", width: 16, height: 16, ft: "Btn", mkCA: "H", onName: "Yes", value: "Yes"},
		{label: "Radio button", width: 16, height: 16, ft: "Btn", ff: acroform.FieldRadio, mkCA: "l", opts: []string{"A", "B", "C"}, value: "A"},
		{label: "Text, single line", width: 150, height: 20, ft: "Tx", value: "Ada Lovelace"},
		{label: "Text, right aligned", width: 150, height: 20, ft: "Tx", align: pdf.TextAlignRight, value: "1843", style: "S"},
		{label: "Text, multiline", width: 150, height: 52, ft: "Tx", ff: acroform.FieldMultiline, value: "Notes on the analytical engine and its capacity for computation."},
		{label: "Text, comb", width: 150, height: 22, ft: "Tx", ff: acroform.FieldComb, value: "AB1234", maxLen: 6},
		{label: "Text, comb (centred)", width: 150, height: 22, ft: "Tx", ff: acroform.FieldComb, align: pdf.TextAlignCenter, value: "AB", maxLen: 6},
		{label: "Text, password", width: 150, height: 20, ft: "Tx", ff: acroform.FieldPassword, value: "secret"},
		{label: "List box", width: 150, height: 72, ft: "Ch", opts: fontOpts, sel: []int{2}, value: fontOpts[2]},
		{label: "Combo box", width: 150, height: 22, ft: "Ch", ff: acroform.FieldCombo, opts: fontOpts, value: fontOpts[2]},
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

	page.Out.GetMeta().Catalog.AcroForm = page.RM.StoreDeferred(wr.form)

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
	wr.nextID++
	f := wr.makeField(d)

	// a radio button field shows one widget per option, side by side; every
	// other field has a single widget
	onStates := []pdf.Name{d.onName}
	if d.ft == "Btn" && d.ff&acroform.FieldRadio != 0 {
		onStates = onStates[:0]
		for _, o := range d.opts {
			onStates = append(onStates, pdf.Name(o))
		}
	}

	const widgetGap = 8.0
	for i, onState := range onStates {
		xi := x + float64(i)*(d.width+widgetGap)
		rect := pdf.Rectangle{LLx: xi, LLy: y, URx: xi + d.width, URy: y + d.height}

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

		// the widget is a child of the field and an annotation on the page; a
		// terminal field with a single widget is written as one merged object,
		// shared between the form's field list and the page's annotation list.
		// The form is stored (see createDocument) before the page is closed, so
		// the merge is in place when the page writes the widget.
		widget := annotation.AddWidget(f, rect)
		widget.Common.Flags = annotation.FlagPrint
		widget.Style = mk
		widget.BorderStyle = bs

		isToggle := d.ft == "Btn" && d.ff&acroform.FieldPushbutton == 0
		if genAP {
			// derives the field context from w.Field and selects the toggle
			// appearance matching the field value
			if err := wr.style.AddAppearance(widget); err != nil {
				return err
			}
		} else if isToggle {
			// show the appearance matching the field value
			if onState == pdf.Name(d.value) {
				widget.AppearanceState = onState
			} else {
				widget.AppearanceState = "Off"
			}
		}

		wr.page.Page.Annots = append(wr.page.Page.Annots, widget)
	}
	return nil
}

// makeField builds the typed form field for d as a root of wr.form. Its widget
// is added separately by the caller.
func (wr *writer) makeField(d demo) acroform.Field {
	name := fmt.Sprintf("f%d", wr.nextID)

	var f acroform.Field
	switch d.ft {
	case "Btn":
		// a push button retains no value; check boxes and radio buttons carry
		// the on-state name in V, and radio buttons name their widgets in Opt
		btn := acroform.NewButtonField(name)
		btn.Flags = d.ff
		if d.ff&acroform.FieldPushbutton == 0 {
			btn.V = pdf.Name(d.value)
			if d.ff&acroform.FieldRadio != 0 {
				btn.Opt = d.opts
			}
		}
		f = btn

	case "Tx":
		tx := acroform.NewTextField(name)
		tx.Flags = d.ff
		tx.DefaultAppearance = defaultDA
		tx.Align = d.align
		tx.MaxLen = d.maxLen
		if d.value != "" {
			tx.V = pdf.TextString(d.value)
		}
		f = tx

	case "Ch":
		ch := acroform.NewChoiceField(name)
		ch.Flags = d.ff
		ch.DefaultAppearance = defaultDA
		ch.Align = d.align
		ch.Selected = d.sel
		for _, o := range d.opts {
			ch.Opt = append(ch.Opt, acroform.ChoiceOption{Display: o, Export: o})
		}
		if d.value != "" {
			ch.V = pdf.TextString(d.value)
		}
		f = ch

	default: // "Sig"
		sig := acroform.NewSignatureField(name)
		sig.Flags = d.ff
		f = sig
	}

	wr.form.Fields = append(wr.form.Fields, f)
	return f
}
