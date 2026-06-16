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

package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action"
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
	pageWidth  = 595.276
	pageHeight = 841.890

	margin       = 56.0
	columnGap    = 24.0
	contentLeft  = margin
	contentRight = pageWidth - margin
	contentWidth = contentRight - contentLeft
	halfWidth    = (contentWidth - columnGap) / 2
	rightColumn  = contentLeft + halfWidth + columnGap

	captionGap  = 8.0
	inputHeight = 24.0
	boxSize     = 16.0

	textAppearance = "/Helv 11 Tf 0.13 0.15 0.18 rg"
)

var (
	inkColor    = color.DeviceRGB{0.13, 0.15, 0.18}
	mutedColor  = color.DeviceRGB{0.42, 0.46, 0.50}
	accentColor = color.DeviceRGB{0.16, 0.40, 0.62}
	fieldColor  = color.DeviceRGB{0.98, 0.98, 0.99}
	borderColor = color.DeviceRGB{0.72, 0.76, 0.80}
	paperColor  = color.DeviceRGB{1, 1, 1}

	countries = []string{"United Kingdom", "France", "Germany", "United States", "Japan"}
	tickets   = []string{"Standard", "Student", "Patron"}
)

func main() {
	if err := writeForm("test.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeForm(filename string) error {
	pdfVersion := pdf.V1_7

	page, err := document.CreateSinglePage(filename, document.A4, pdfVersion, &pdf.WriterOptions{HumanReadable: true})
	if err != nil {
		return err
	}

	body := font.Must(standard.Helvetica.New())
	bold := font.Must(standard.HelveticaBold.New())

	b := &formBuilder{
		page: page,
		form: &acroform.InteractiveForm{
			DefaultResources: &content.Resources{
				Font: map[pdf.Name]font.Instance{"Helv": body},
			},
		},
		appearances: fallback.NewStyle(pdfVersion),
		border:      &annotation.BorderStyle{Width: 1},
		chrome:      &appearance.Characteristics{BackgroundColor: fieldColor, BorderColor: borderColor},
		body:        body,
		bold:        bold,
	}

	b.build()
	if b.err != nil {
		return b.err
	}

	formRef, err := page.RM.Store(b.form)
	if err != nil {
		return err
	}
	page.Out.GetMeta().Catalog.AcroForm = formRef

	return page.Close()
}

type formBuilder struct {
	page        *document.Page
	form        *acroform.InteractiveForm
	appearances *fallback.Style
	border      *annotation.BorderStyle
	chrome      *appearance.Characteristics
	body        font.Layouter
	bold        font.Layouter
	y           float64
	err         error
}

func (b *formBuilder) build() {
	b.banner("Spring Symposium 2026", "Attendee registration")
	b.y = pageHeight - 96 - 38

	b.section("Attendee")

	top := b.captions("Given name", "Family name")
	b.addText("given_name", "Ada", 0, box(contentLeft, top, halfWidth, inputHeight))
	b.addText("family_name", "Lovelace", 0, box(rightColumn, top, halfWidth, inputHeight))
	b.advance(inputHeight)

	top = b.captions("Email address")
	b.addText("email", "ada@example.com", 0, box(contentLeft, top, contentWidth, inputHeight))
	b.advance(inputHeight)

	top = b.captions("Country", "Affiliation")
	b.addCombo("country", countries, "United Kingdom", box(contentLeft, top, halfWidth, inputHeight))
	b.addText("affiliation", "Analytical Society", 0, box(rightColumn, top, halfWidth, inputHeight))
	b.advance(inputHeight)

	b.y -= 18
	b.section("Registration")

	top = b.captions("Ticket type")
	b.addRadio("ticket", tickets, "Patron", top, 150)
	b.advance(boxSize)

	top = b.captions("Preferences")
	b.addCheckBox("vegetarian", "Vegetarian meals", false, contentLeft, top)
	b.addCheckBox("newsletter", "Subscribe to newsletter", true, contentLeft+230, top)
	b.advance(boxSize)

	b.y -= 18
	b.section("Comments")

	top = b.captions("Anything we should know?")
	b.addText("comments", "I would like a seat near the front, please.",
		acroform.FieldMultiline, box(contentLeft, top, contentWidth, 96))
	b.advance(96)

	b.buttons()
	b.footer()
}

func (b *formBuilder) addText(name, value string, flags acroform.FieldFlags, rect pdf.Rectangle) {
	f := acroform.NewTextField(name)
	f.Ff = flags
	f.DefaultAppearance = textAppearance
	if value != "" {
		f.V = pdf.TextString(value)
	}
	b.form.Fields = append(b.form.Fields, f)
	b.attach(f, rect, b.chrome)
}

func (b *formBuilder) addCombo(name string, options []string, value string, rect pdf.Rectangle) {
	f := acroform.NewChoiceField(name)
	f.Ff = acroform.FieldCombo
	f.DefaultAppearance = textAppearance
	for _, option := range options {
		f.Opt = append(f.Opt, acroform.ChoiceOption{Display: option, Export: option})
	}
	f.V = pdf.TextString(value)
	b.form.Fields = append(b.form.Fields, f)
	b.attach(f, rect, b.chrome)
}

func (b *formBuilder) addCheckBox(name, label string, checked bool, x, top float64) {
	f := acroform.NewButtonField(name)
	if checked {
		f.V = "Yes"
	}
	b.form.Fields = append(b.form.Fields, f)

	mk := fieldChrome()
	mk.Caption = "4"
	b.attach(f, box(x, top, boxSize, boxSize), mk)
	b.inlineLabel(label, x+boxSize+7, top-boxSize+4)
}

func (b *formBuilder) addRadio(name string, options []string, selected pdf.Name, top, slot float64) {
	f := acroform.NewButtonField(name)
	f.Ff = acroform.FieldRadio
	f.Opt = options
	f.V = selected
	b.form.Fields = append(b.form.Fields, f)

	for i, option := range options {
		x := contentLeft + float64(i)*slot
		mk := fieldChrome()
		mk.Caption = "l"
		b.attach(f, box(x, top, boxSize, boxSize), mk)
		b.inlineLabel(option, x+boxSize+7, top-boxSize+4)
	}
}

func (b *formBuilder) buttons() {
	top := b.y + 8
	const width, gap = 120.0, 16.0
	b.addButton("reset", "Reset", borderColor, &action.ResetForm{}, box(contentRight-2*width-gap, top, width, 30))
	b.addButton("submit", "Submit", accentColor, &action.SubmitForm{F: pdf.String("https://symposium.example/register")}, box(contentRight-width, top, width, 30))
}

func (b *formBuilder) addButton(name, caption string, border color.Color, act pdf.Action, rect pdf.Rectangle) {
	f := acroform.NewButtonField(name)
	f.Ff = acroform.FieldPushbutton
	b.form.Fields = append(b.form.Fields, f)

	mk := &appearance.Characteristics{BackgroundColor: fieldColor, BorderColor: border, Caption: caption}
	if w := b.attach(f, rect, mk); w != nil {
		w.Action = act
	}
}

func (b *formBuilder) attach(field acroform.Field, rect pdf.Rectangle, mk *appearance.Characteristics) *annotation.Widget {
	if b.err != nil {
		return nil
	}
	w := annotation.AddWidget(field, rect)
	w.Common.Flags = annotation.FlagPrint
	w.MK = mk
	w.BorderStyle = b.border
	if err := b.appearances.AddAppearance(w); err != nil {
		b.err = err
		return nil
	}
	b.page.Page.Annots = append(b.page.Page.Annots, w)
	return w
}

func fieldChrome() *appearance.Characteristics {
	return &appearance.Characteristics{BackgroundColor: fieldColor, BorderColor: borderColor}
}

func box(x, top, width, height float64) pdf.Rectangle {
	return pdf.Rectangle{LLx: x, LLy: top - height, URx: x + width, URy: top}
}

func (b *formBuilder) advance(height float64) {
	b.y -= height + 38
}

func (b *formBuilder) captions(labels ...string) float64 {
	columns := []float64{contentLeft, rightColumn}
	for i, label := range labels {
		b.text(b.body, 8.5, mutedColor, label, columns[i], b.y)
	}
	return b.y - captionGap
}

func (b *formBuilder) inlineLabel(label string, x, baseline float64) {
	b.text(b.body, 10, inkColor, label, x, baseline)
}

func (b *formBuilder) section(title string) {
	b.text(b.bold, 12, inkColor, title, contentLeft, b.y)
	b.rule(b.y-7, borderColor, 0.75)
	b.y -= 30
}

func (b *formBuilder) banner(title, subtitle string) {
	p := b.page
	p.SetFillColor(accentColor)
	p.Rectangle(0, pageHeight-96, pageWidth, 96)
	p.Fill()
	b.text(b.bold, 22, paperColor, title, margin, pageHeight-54)
	b.text(b.body, 11, color.DeviceRGB{0.86, 0.91, 0.96}, subtitle, margin, pageHeight-74)
}

func (b *formBuilder) footer() {
	b.rule(72, borderColor, 0.5)
	b.text(b.body, 8, mutedColor, "Spring Symposium 2026  ·  generated with Quire", margin, 60)
}

func (b *formBuilder) rule(y float64, c color.Color, width float64) {
	p := b.page
	p.SetStrokeColor(c)
	p.SetLineWidth(width)
	p.MoveTo(margin, y)
	p.LineTo(pageWidth-margin, y)
	p.Stroke()
}

func (b *formBuilder) text(f font.Layouter, size float64, c color.Color, s string, x, baseline float64) {
	p := b.page
	p.TextBegin()
	p.TextSetFont(f, size)
	p.SetFillColor(c)
	p.TextFirstLine(x, baseline)
	p.TextShow(s)
	p.TextEnd()
}
