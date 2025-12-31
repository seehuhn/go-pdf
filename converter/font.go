package converter

import (
	"fmt"
	"io"
	"math"
	"strings"

	"seehuhn.de/go/pdf/font"
)

// FontTracker tracks unique fonts used in the document.
type FontTracker struct {
	Fonts []font.Instance
	Sizes []float64
}

// AddFont adds a font and size to the tracker if not already present.
// Returns the index of the font.
func (ft *FontTracker) AddFont(f font.Instance, size float64) int {
	name := "sans-serif"
	if f != nil {
		name = f.PostScriptName()
	}
	name = strings.ToLower(name)
	roundedSize := math.Round(size)

	for i, existingFont := range ft.Fonts {
		existingName := "sans-serif"
		if existingFont != nil {
			existingName = existingFont.PostScriptName()
		}
		existingName = strings.ToLower(existingName)
		existingRounded := math.Round(ft.Sizes[i])

		if name == existingName && existingRounded == roundedSize {
			return i
		}
	}
	ft.Fonts = append(ft.Fonts, f)
	ft.Sizes = append(ft.Sizes, size)
	return len(ft.Fonts) - 1
}

// WriteCSS writes the CSS for all tracked fonts.
func (ft *FontTracker) WriteCSS(out io.Writer) error {
	if _, err := fmt.Fprintln(out, "<style type=\"text/css\">"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  body { background-color: #f0f0f0; }"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  div { background-color: white; margin-bottom: 20px; box-shadow: 2px 2px 5px rgba(0,0,0,0.1); }"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  p { margin: 0; padding: 0; }"); err != nil {
		return err
	}

	for i, f := range ft.Fonts {
		size := ft.Sizes[i]
		// Determine font family name
		name := "sans-serif"
		if f != nil {
			name = f.PostScriptName()
		}

		style := ""
		lname := strings.ToLower(name)
		if strings.Contains(lname, "bold") {
			style += " font-weight: bold;"
		}
		if strings.Contains(lname, "italic") || strings.Contains(lname, "oblique") {
			style += " font-style: italic;"
		}

		_, err := fmt.Fprintf(out, "  .ft%d { font-family: %q; font-size: %dpx;%s }\n", i, name, int(size), style)
		if err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "</style>"); err != nil {
		return err
	}
	return nil
}
