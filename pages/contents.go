package pages

import (
	"bufio"
	"io"

	"seehuhn.de/go/pdf"
)

// Page represents the contents of a page in the PDF file.  The object provides
// .Write() and .WriteString() methods to write the PDF content stream for the
// page.  The .Close() method must be called after the content stream has been
// written completely.
type Page struct {
	LLx, LLy, URx, URy float64 // The media box for the page

	w   *bufio.Writer
	stm io.WriteCloser
}

// AddPage adds a new page to the page tree and returns an object which
// can be used to write the content stream for the page.
func (tree *PageTree) AddPage(attr *Attributes) (*Page, error) {
	var mediaBox *Rectangle
	def := tree.defaults
	if def != nil {
		mediaBox = def.MediaBox
	}
	if attr != nil && attr.MediaBox != nil {
		mediaBox = attr.MediaBox
	}

	contentRef := tree.w.Alloc()

	pageDict := pdf.Dict{
		"Type":     pdf.Name("Page"),
		"Contents": contentRef,
	}
	if attr != nil {
		if len(attr.Resources) > 0 {
			pageDict["Resources"] = attr.Resources
		}
		if attr.MediaBox != nil &&
			(def.MediaBox == nil || !def.MediaBox.NearlyEqual(attr.MediaBox, 1)) {
			pageDict["MediaBox"] = attr.MediaBox.ToObject()
		}
		if attr.CropBox != nil &&
			(def.CropBox == nil || !def.CropBox.NearlyEqual(attr.CropBox, 1)) {
			pageDict["CropBox"] = attr.CropBox.ToObject()
		}
		if attr.Rotate != 0 && def.Rotate != attr.Rotate {
			pageDict["Rotate"] = pdf.Integer(attr.Rotate)
		}
	}
	err := tree.Ship(pageDict, nil)
	if err != nil {
		return nil, err
	}

	// TODO(voss): compress the stream
	stream, _, err := tree.w.OpenStream(nil, contentRef, nil)
	if err != nil {
		return nil, err
	}
	return &Page{
		LLx: mediaBox.LLx,
		LLy: mediaBox.LLy,
		URx: mediaBox.URx,
		URy: mediaBox.URy,

		w:   bufio.NewWriter(stream),
		stm: stream,
	}, nil
}

// Write writes the contents of buf to the content stream.  It returns the
// number of bytes written.  If nn < len(p), it also returns an error
// explaining why the write is short.
func (p *Page) Write(buf []byte) (int, error) {
	return p.w.Write(buf)
}

// WriteString appends a string to the content stream.  It returns the number
// of bytes written.  If the count is less than len(s), it also returns an
// error explaining why the write is short.
func (p *Page) WriteString(s string) (int, error) {
	return p.w.WriteString(s)
}

// Close writes any buffered data to the content stream and the closes the
// stream.  The Page object cannot be used any more after .Close() has been
// called.
func (p *Page) Close() error {
	err := p.w.Flush()
	if err != nil {
		return err
	}
	p.w = nil
	return p.stm.Close()
}
