package walker

import (
	"errors"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf"
)

// mockGetter simulates a simple PDF structure for testing
type mockGetter struct {
	objects map[pdf.Reference]pdf.Native
	meta    *pdf.MetaInfo
}

var (
	mockErrorRef = pdf.Reference(99)
	errMock      = errors.New("mock error")

	finalRef = pdf.Reference(42)
)

func (m *mockGetter) Get(ref pdf.Reference, _ bool) (pdf.Native, error) {
	if ref == mockErrorRef {
		return nil, errMock
	}
	return m.objects[ref], nil
}

func (m *mockGetter) GetMeta() *pdf.MetaInfo {
	return m.meta
}

func newMockPDF() *mockGetter {
	return &mockGetter{
		objects: map[pdf.Reference]pdf.Native{
			pdf.Reference(1): pdf.Name("unused object"),
			pdf.Reference(2): pdf.Dict{
				"Type": pdf.Name("Pages"),
				"Kids": pdf.Array{pdf.Reference(3), pdf.Reference(4)},
			},
			pdf.Reference(3): pdf.Dict{
				"Type":     pdf.Name("Page"),
				"Parent":   pdf.Reference(2),
				"Contents": pdf.Reference(5),
			},
			pdf.Reference(4): pdf.Dict{ // object 4
				"Type":     pdf.Name("Page"),
				"Parent":   pdf.Reference(2),
				"Contents": pdf.Reference(6),
			},
			pdf.Reference(5): pdf.String("Content of page 1"),
			pdf.Reference(6): pdf.String("Content of page 2"),
			finalRef:         pdf.String("Final object"),
		},
		meta: &pdf.MetaInfo{
			Info: &pdf.Info{
				Title: "Mock PDF",
			},
			Catalog: &pdf.Catalog{
				Pages: pdf.Reference(2),
			},
			Trailer: pdf.Dict{
				"Final": finalRef,
			},
		},
	}
}

func TestWalker_PreOrder(t *testing.T) {
	mockPDF := newMockPDF()
	w := New(mockPDF)

	var objects []pdf.Reference
	for ref, _ := range w.PreOrder() {
		if ref != 0 {
			objects = append(objects, ref)
		}
	}

	if w.Err != nil {
		t.Errorf("Unexpected error: %v", w.Err)
	}

	expected := []pdf.Reference{2, 3, 5, 4, 6, finalRef}
	if !reflect.DeepEqual(objects, expected) {
		t.Errorf("Incorrect pre-order traversal. Got %v, want %v", objects, expected)
	}
}

func TestWalker_PostOrder(t *testing.T) {
	mockPDF := newMockPDF()
	w := New(mockPDF)

	var objects []pdf.Reference
	for ref, _ := range w.PostOrder() {
		if ref != 0 {
			objects = append(objects, ref)
		}
	}

	if w.Err != nil {
		t.Errorf("Unexpected error: %v", w.Err)
	}

	expected := []pdf.Reference{5, 3, 6, 4, 2, finalRef}
	if !reflect.DeepEqual(objects, expected) {
		t.Errorf("Incorrect post-order traversal. Got %v, want %v", objects, expected)
	}
}

func TestWalker_Error(t *testing.T) {
	mockPDF := newMockPDF()

	// add a reference to broken object to the catalog
	mockPDF.meta.Catalog.Metadata = mockErrorRef

	w := New(mockPDF)

	for ref := range w.PreOrder() {
		if ref == mockErrorRef {
			t.Errorf("mockErrorRef should not be reached")
		}
		if ref == finalRef {
			t.Errorf("finalRef should not be reached")
		}
	}

	if w.Err != errMock {
		t.Errorf("expected errMock, got %v", w.Err)
	}
}
