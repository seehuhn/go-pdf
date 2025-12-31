package converter

import (
	"math"
	"slices"
	"strings"
)

// TextDirection represents the direction of the text.
type TextDirection uint8

const (
	TextDirUnknown TextDirection = iota
	TextDirLeftToRight
	TextDirRightToLeft
	TextDirTopToBottom
)

// TextRun represents a sequence of characters with the same style.
type TextRun struct {
	Text   string
	FontID int
}

// Fragment represents a logical block of text (like a line or paragraph).
// It contains one or more TextRuns, allowing for style changes within the block.
type Fragment struct {
	Runs       []TextRun
	XMin, XMax float64
	YMin, YMax float64
	Dir        TextDirection

	// Internal links or other metadata could go here
}

// AddRun adds a text run to the fragment.
func (f *Fragment) AddRun(text string, fontID int) {
	if len(f.Runs) > 0 && f.Runs[len(f.Runs)-1].FontID == fontID {
		f.Runs[len(f.Runs)-1].Text += text
		return
	}
	f.Runs = append(f.Runs, TextRun{Text: text, FontID: fontID})
}

// FullText returns the combined text of all runs.
func (f *Fragment) FullText() string {
	var sb strings.Builder
	for _, r := range f.Runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// Page represents a single page of text fragments.
// Equivalent to Poppler's HtmlPage.
type Page struct {
	Fragments []*Fragment
	Width     float64
	Height    float64
	PageNum   int
	RawOrder  bool
}

// NewPage creates a new Page.
func NewPage(pageNum int, width, height float64, rawOrder bool) *Page {
	return &Page{
		PageNum:  pageNum,
		Width:    width,
		Height:   height,
		RawOrder: rawOrder,
	}
}

// AddFragment adds a text fragment to the page.
func (p *Page) AddFragment(f *Fragment) {
	p.Fragments = append(p.Fragments, f)
}

// Sort sorts the fragments in y-major, then x-major order.
func (p *Page) Sort() {
	slices.SortFunc(p.Fragments, func(a, b *Fragment) int {
		ha := a.YMax - a.YMin
		ya := a.YMin + 0.5*ha

		hb := b.YMax - b.YMin
		yb := b.YMin + 0.5*hb

		if math.Abs(ya-yb) < 1.0 { // Increased tolerance
			if a.XMin < b.XMin {
				return -1
			}
			if a.XMin > b.XMin {
				return 1
			}
			return 0
		}
		if ya < yb {
			return -1
		}
		return 1
	})
}

// Coalesce merges nearby fragments into larger blocks.
// This is a simplified version of Poppler's HtmlPage::coalesce.
func (p *Page) Coalesce() {
	if len(p.Fragments) < 2 {
		return
	}

	p.Sort()

	var lines []*Fragment
	current := p.Fragments[0]

	for i := 1; i < len(p.Fragments); i++ {
		next := p.Fragments[i]
		space := current.YMax - current.YMin
		horSpace := next.XMin - current.XMax

		vertOverlap := 0.0
		if next.YMin >= current.YMin && next.YMin <= current.YMax {
			vertOverlap = current.YMax - next.YMin
		} else if next.YMax >= current.YMin && next.YMax <= current.YMax {
			vertOverlap = next.YMax - current.YMin
		}

		// Strictly same line
		sameLine := (vertOverlap > 0.5*space) && (horSpace > -0.5*space && horSpace < 4.0*space)

		if sameLine {
			if horSpace > 0.05*space {
				current.AddRun(" ", current.Runs[len(current.Runs)-1].FontID)
			}
			for _, run := range next.Runs {
				current.AddRun(run.Text, run.FontID)
			}
			current.XMax = math.Max(current.XMax, next.XMax)
			current.XMin = math.Min(current.XMin, next.XMin)
			current.YMax = math.Max(current.YMax, next.YMax)
			current.YMin = math.Min(current.YMin, next.YMin)
		} else {
			lines = append(lines, current)
			current = next
		}
	}
	lines = append(lines, current)

	// Pass 2: Merge subsequent lines into paragraphs.
	var paragraphs []*Fragment
	current = lines[0]

	for i := 1; i < len(lines); i++ {
		next := lines[i]
		space := next.YMax - next.YMin
		vertSpace := next.YMin - current.YMax

		// Heuristic for "next line of same paragraph"
		// We allow some indentation delta (5.0px) or same left edge
		addLineBreak := math.Abs(current.XMin-next.XMin) < 3.0 &&
			vertSpace >= 0 && vertSpace < 1.2*space

		// Special case: lists. If the current line is short and next is indented, maybe?
		// But for now, let's stick to simple vertical proximity and left alignment.

		if addLineBreak {
			current.AddRun("\n", current.Runs[len(current.Runs)-1].FontID)
			for _, run := range next.Runs {
				current.AddRun(run.Text, run.FontID)
			}
			current.XMax = math.Max(current.XMax, next.XMax)
			current.XMin = math.Min(current.XMin, next.XMin)
			current.YMax = math.Max(current.YMax, next.YMax)
			current.YMin = math.Min(current.YMin, next.YMin)
		} else {
			paragraphs = append(paragraphs, current)
			current = next
		}
	}
	paragraphs = append(paragraphs, current)
	p.Fragments = paragraphs
}
