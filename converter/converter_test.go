package converter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestConversionFixtures(t *testing.T) {
	pdfPath := filepath.Join("..", "testdata", "fixtures", "page0002.pdf")
	absHTMLPath := filepath.Join("..", "testdata", "fixtures", "page_absolute.html")
	flowHTMLPath := filepath.Join("..", "testdata", "fixtures", "page_flow.html")

	// 1. Read PDF
	f, err := os.Open(pdfPath)
	if err != nil {
		t.Fatalf("failed to open PDF fixture: %v", err)
	}
	defer f.Close()

	r, err := pdf.NewReader(f, nil)
	if err != nil {
		t.Fatalf("failed to create PDF reader: %v", err)
	}

	// 2. Convert to Absolute HTML
	testMode(t, r, absHTMLPath, false)

	// 3. Convert to Flow HTML
	testMode(t, r, flowHTMLPath, true)
}

func testMode(t *testing.T, r pdf.Getter, fixturePath string, flowMode bool) {
	c := NewConverter(r)
	pages, err := c.ConvertDocument()
	if err != nil {
		t.Fatalf("conversion failed (flow=%v): %v", flowMode, err)
	}

	var buf bytes.Buffer
	hw := NewHTMLWriter(&buf, c.Tracker)
	hw.FlowMode = flowMode

	if err := hw.WriteHeader(); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	for _, p := range pages {
		if err := hw.WritePage(p); err != nil {
			t.Fatalf("failed to write page: %v", err)
		}
	}
	if err := hw.WriteFooter(); err != nil {
		t.Fatalf("failed to write footer: %v", err)
	}

	actual := buf.String()

	// Read fixture
	expectedBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", fixturePath, err)
	}
	expected := string(expectedBytes)

	// Compare normalized strings (ignore minor whitespace/newline differences if any)
	if normalize(actual) != normalize(expected) {
		// For debugging, write temporary actual file
		actualPath := fixturePath + ".actual"
		os.WriteFile(actualPath, buf.Bytes(), 0644)
		t.Errorf("output does not match fixture %s. actual written to %s", fixturePath, actualPath)
	}
}

func normalize(s string) string {
	// Simple normalization: trim space and normalize newlines
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var result []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}
