package converter

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// HTMLWriter handles writing the layout structures to HTML.
type HTMLWriter struct {
	Out      io.Writer
	Tracker  *FontTracker
	FlowMode bool
}

// NewHTMLWriter creates a new HTMLWriter.
func NewHTMLWriter(out io.Writer, tracker *FontTracker) *HTMLWriter {
	return &HTMLWriter{Out: out, Tracker: tracker}
}

// WritePage writes a single page to HTML.
func (h *HTMLWriter) WritePage(p *Page) error {
	divStyle := ""
	if !h.FlowMode {
		divStyle = fmt.Sprintf("position:relative;width:%dpx;height:%dpx;", int(p.Width), int(p.Height))
	}
	fmt.Fprintf(h.Out, "<div id=\"page%d-div\" style=\"%s\">\n", p.PageNum, divStyle)

	for _, f := range p.Fragments {
		style := ""
		if !h.FlowMode {
			style = fmt.Sprintf("position:absolute;top:%dpx;left:%dpx;", int(f.YMin), int(f.XMin))
		}

		fullText := f.FullText()
		ws := "white-space:nowrap"
		if strings.Contains(fullText, "\n") {
			ws = "white-space:pre-wrap"
		}

		fmt.Fprintf(h.Out, "  <p style=\"%s%s\">", style, ws)
		for _, run := range f.Runs {
			txt := html.EscapeString(run.Text)
			txt = strings.ReplaceAll(txt, "\n", "<br/>")
			fmt.Fprintf(h.Out, "<span class=\"ft%d\">%s</span>", run.FontID, txt)
		}
		fmt.Fprintf(h.Out, "</p>\n")
	}

	fmt.Fprintln(h.Out, "</div>")
	return nil
}

// WriteHeader writes the HTML header.
func (h *HTMLWriter) WriteHeader() error {
	fmt.Fprintln(h.Out, "<!DOCTYPE html>")
	fmt.Fprintln(h.Out, "<html>")
	fmt.Fprintln(h.Out, "<head>")
	fmt.Fprintln(h.Out, "<meta charset=\"utf-8\">")
	h.Tracker.WriteCSS(h.Out)
	fmt.Fprintln(h.Out, "</head>")
	fmt.Fprintln(h.Out, "<body>")
	return nil
}

// WriteFooter writes the HTML footer.
func (h *HTMLWriter) WriteFooter() error {
	fmt.Fprintln(h.Out, "</body>")
	fmt.Fprintln(h.Out, "</html>")
	return nil
}
