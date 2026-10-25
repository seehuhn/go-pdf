// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package file

import (
	"bytes"
	"errors"
	"io"
	"maps"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/collection"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/image/thumbnail"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

var testCases = []struct {
	name    string
	spec    *Specification
	version pdf.Version
}{
	{
		name:    "basic file specification",
		version: pdf.V1_0,
		spec: &Specification{
			FileName:       "example.txt",
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "single use file specification",
		version: pdf.V1_0,
		spec: &Specification{
			FileName:       "inline.txt",
			AFRelationship: RelationshipUnspecified,
			SingleUse:      true,
		},
	},
	{
		name:    "file specification with description",
		version: pdf.V1_6,
		spec: &Specification{
			FileName:       "document.pdf",
			Description:    pdf.Text("Important document"),
			Volatile:       true,
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with unicode name",
		version: pdf.V1_7,
		spec: &Specification{
			FileName:        "example.txt",
			FileNameUnicode: pdf.Text("example_unicode.txt"),
			Description:     pdf.Text("Unicode filename test"),
			ID:              []string{"id1", "id2"},
			AFRelationship:  RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with all legacy names",
		version: pdf.V1_0,
		spec: &Specification{
			FileName:       "example.txt",
			FileNameDOS:    pdf.String("EXAMPLE.TXT"),
			FileNameMac:    pdf.String("example.txt"),
			FileNameUnix:   pdf.String("example.txt"),
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with embedded files",
		version: pdf.V1_3,
		spec: &Specification{
			FileName: "document.pdf",
			EmbeddedFiles: map[string]*Stream{
				"F": {
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("Document F content"))
						return err
					},
				},
				"UF": {
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("Document UF content"))
						return err
					},
				},
			},
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with related files",
		version: pdf.V1_3,
		spec: &Specification{
			FileName: "main.txt",
			EmbeddedFiles: map[string]*Stream{
				"F": {
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("Main file content"))
						return err
					},
				},
			},
			RelatedFiles: map[string][]RelatedFile{
				"F": {
					{Name: pdf.Text("related1.txt"), Stream: &Stream{
						WriteData: func(w io.Writer) error {
							_, err := w.Write([]byte("Related file 1 content"))
							return err
						},
					}},
					{Name: pdf.Text("related2.txt"), Stream: &Stream{
						WriteData: func(w io.Writer) error {
							_, err := w.Write([]byte("Related file 2 content"))
							return err
						},
					}},
				},
			},
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with encrypted payload",
		version: pdf.V2_0,
		spec: &Specification{
			FileName: "encrypted.pdf",
			EncryptedPayload: &EncryptedPayload{
				FilterName: "CustomCrypto",
				Version:    "1.0",
			},
			AFRelationship: RelationshipEncryptedPayload,
		},
	},
	{
		name:    "file specification with collection item",
		version: pdf.V1_7,
		spec: &Specification{
			FileName: "collection_item.pdf",
			CollectionItem: &collection.ItemDict{
				Data: map[pdf.Name]collection.ItemValue{
					"Title":   {Val: pdf.Text("Test Collection Item")},
					"Created": {Val: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
				},
			},
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with thumbnail",
		version: pdf.V2_0,
		spec: &Specification{
			FileName: "image.jpg",
			Thumbnail: &thumbnail.Thumbnail{
				Width:            2,
				Height:           2,
				ColorSpace:       color.SpaceDeviceGray,
				BitsPerComponent: 8,
				Source: thumbnail.NewRawSource(func(w io.Writer) error {
					_, err := w.Write([]byte{0x00, 0x80, 0x80, 0xff})
					return err
				}),
			},
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "url file specification",
		version: pdf.V1_0,
		spec: &Specification{
			NameSpace:      "URL",
			FileName:       "https://example.com/file.pdf",
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "comprehensive file specification",
		version: pdf.V2_0,
		spec: &Specification{
			NameSpace:       "URL",
			FileName:        "https://example.com/document.pdf",
			FileNameUnicode: pdf.Text("https://example.com/document_unicode.pdf"),
			Description:     pdf.Text("Comprehensive test document"),
			ID:              []string{"comp1", "comp2"},
			Volatile:        true,
			EmbeddedFiles: map[string]*Stream{
				"F": {
					MimeType:     "application/pdf",
					Size:         2048,
					CreationDate: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
					ModDate:      time.Date(2023, 6, 15, 14, 30, 0, 0, time.UTC),
					CheckSum:     []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("Comprehensive F content with full metadata"))
						return err
					},
				},
				"UF": {
					MimeType: "text/plain",
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("Unicode filename content"))
						return err
					},
				},
			},
			RelatedFiles: map[string][]RelatedFile{
				"F": {
					{Name: pdf.Text("metadata.xml"), Stream: &Stream{
						MimeType: "text/xml",
						WriteData: func(w io.Writer) error {
							_, err := w.Write([]byte("<?xml version=\"1.0\"?><metadata></metadata>"))
							return err
						},
					}},
				},
			},
			AFRelationship: RelationshipSource,
			CollectionItem: &collection.ItemDict{
				Data: map[pdf.Name]collection.ItemValue{
					"Document": {Val: pdf.Text("Comprehensive Test Document")},
					"Size":     {Val: int64(1024), Prefix: pdf.Text("Size: ")},
					"Rating":   {Val: float64(4.8)},
				},
			},
			Thumbnail: &thumbnail.Thumbnail{
				Width:            1,
				Height:           1,
				ColorSpace:       color.SpaceDeviceRGB,
				BitsPerComponent: 16,
				Source: thumbnail.NewRawSource(func(w io.Writer) error {
					_, err := w.Write([]byte{0x00, 0x00, 0xff, 0xff, 0x00, 0x00})
					return err
				}),
			},
			EncryptedPayload: &EncryptedPayload{
				FilterName: "AdvancedCrypto",
				Version:    "2.1",
			},
		},
	},
}

func TestSpecificationRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, tc.version, nil)
			rm := pdf.NewResourceManager(w)

			// Embed the specification
			obj, err := rm.Embed(tc.spec)
			if err != nil {
				t.Fatal(err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}
			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			// Extract it back
			x := pdf.NewExtractor(w)
			decoded, err := pdf.CursorAt(x, nil).Decode(obj, ExtractSpecification)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.spec, decoded, cmp.Comparer(func(a, b *Stream) bool {
				if a == nil && b == nil {
					return true
				}
				if a == nil || b == nil {
					return false
				}
				return a.Equal(b)
			})); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSpecificationValidation(t *testing.T) {
	t.Run("missing all file names", func(t *testing.T) {
		spec := &Specification{
			Description: pdf.Text("No filenames"),
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V1_0, nil)
		rm := pdf.NewResourceManager(buf)

		_, err := rm.Embed(spec)
		if err == nil {
			t.Error("expected error for missing file names")
		}
	})

	t.Run("Unicode file name only", func(t *testing.T) {
		spec := &Specification{FileNameUnicode: pdf.Text("test.txt")}

		buf, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		rm := pdf.NewResourceManager(buf)

		_, err := rm.Embed(spec)
		if err == nil {
			t.Error("expected error for UF without F")
		}
	})

	t.Run("version requirement - UF", func(t *testing.T) {
		spec := &Specification{
			FileName:        "test.txt",
			FileNameUnicode: pdf.Text("test_unicode.txt"),
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V1_6, nil)
		rm := pdf.NewResourceManager(buf)

		_, err := rm.Embed(spec)
		if err == nil {
			t.Error("expected version error for UF in PDF 1.6")
		}
	})

	t.Run("version requirement - EF", func(t *testing.T) {
		spec := &Specification{
			FileName: "test.txt",
			EmbeddedFiles: map[string]*Stream{
				"F": {
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("test content"))
						return err
					},
				},
			},
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V1_2, nil)
		rm := pdf.NewResourceManager(buf)

		_, err := rm.Embed(spec)
		if err == nil {
			t.Error("expected version error for EF in PDF 1.2")
		}
	})

	t.Run("version requirement - PDF 2.0", func(t *testing.T) {
		spec := &Specification{
			FileName: "test.txt",
			Thumbnail: &thumbnail.Thumbnail{
				Width:            1,
				Height:           1,
				ColorSpace:       color.SpaceDeviceGray,
				BitsPerComponent: 8,
				Source: thumbnail.NewRawSource(func(w io.Writer) error {
					_, err := w.Write([]byte{0x00})
					return err
				}),
			},
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		rm := pdf.NewResourceManager(buf)

		_, err := rm.Embed(spec)
		if err == nil {
			t.Error("expected version error for Thumbnail in PDF 1.7")
		}
	})
}

func TestSpecificationIndirectReference(t *testing.T) {
	t.Run("EF requires indirect reference", func(t *testing.T) {
		spec := &Specification{
			FileName: "test.txt",
			EmbeddedFiles: map[string]*Stream{
				"F": {
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("test content"))
						return err
					},
				},
			},
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V1_3, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := rm.Embed(spec)
		if err != nil {
			t.Fatal(err)
		}

		// Should return a reference, not a direct dictionary
		if _, isRef := obj.(pdf.Reference); !isRef {
			t.Error("expected indirect reference when EF is present")
		}
	})

	t.Run("RF requires indirect reference", func(t *testing.T) {
		spec := &Specification{
			FileName: "test.txt",
			RelatedFiles: map[string][]RelatedFile{
				"F": {{Name: pdf.Text("related.txt"), Stream: &Stream{
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("Related file content"))
						return err
					},
				}}},
			},
			SingleUse: true, // Try to get a direct dictionary
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V1_3, nil)
		rm := pdf.NewResourceManager(buf)

		// Should return an error because RF requires indirect reference
		_, err := rm.Embed(spec)
		if err == nil {
			t.Error("expected error when RF is present with SingleUse=true")
		}
	})

	t.Run("simple spec can be direct", func(t *testing.T) {
		spec := &Specification{
			FileName:  "test.txt",
			SingleUse: true, // Request a direct dictionary
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V1_0, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := rm.Embed(spec)
		if err != nil {
			t.Fatal(err)
		}

		// Should return a direct dictionary
		if _, isDict := obj.(pdf.Dict); !isDict {
			t.Error("expected direct dictionary for simple specification")
		}
	})
}

func TestSpecificationAFRelationship(t *testing.T) {
	t.Run("default relationship", func(t *testing.T) {
		spec := &Specification{
			FileName:       "test.txt",
			AFRelationship: RelationshipUnspecified,
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := rm.Embed(spec)
		if err != nil {
			t.Fatal(err)
		}

		dict, err := pdf.NewCursor(buf).Dict(obj)
		if err != nil {
			t.Fatal(err)
		}

		// Unspecified should not be written (it's the default)
		if _, hasAF := dict["AFRelationship"]; hasAF {
			t.Error("AFRelationship should not be written when Unspecified")
		}
	})

	t.Run("explicit relationship", func(t *testing.T) {
		spec := &Specification{
			FileName:       "test.txt",
			AFRelationship: RelationshipSource,
		}

		buf, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := rm.Embed(spec)
		if err != nil {
			t.Fatal(err)
		}

		dict, err := pdf.NewCursor(buf).Dict(obj)
		if err != nil {
			t.Fatal(err)
		}

		if dict["AFRelationship"] != pdf.Name("Source") {
			t.Error("AFRelationship should be written when not Unspecified")
		}
	})
}

func TestSpecificationMalformedInput(t *testing.T) {
	t.Run("malformed RF dictionary", func(t *testing.T) {
		buf, _ := memfile.NewPDFWriter(t, pdf.V1_3, nil)
		x := pdf.NewExtractor(buf)

		// Create malformed dictionary with invalid RF structure
		dict := pdf.Dict{
			"Type": pdf.Name("Filespec"),
			"F":    pdf.Text("test.txt"),
			"RF": pdf.Dict{
				"F": pdf.String("not_an_array"), // Should be array
			},
		}

		// Should handle malformed RF gracefully
		_, err := ExtractSpecification(pdf.CursorAt(x, nil), dict, true)
		if err != nil {
			t.Errorf("should handle malformed RF gracefully: %v", err)
		}
	})

	t.Run("malformed ID array", func(t *testing.T) {
		buf, _ := memfile.NewPDFWriter(t, pdf.V1_0, nil)
		x := pdf.NewExtractor(buf)

		// Create dictionary with malformed ID (only one element)
		dict := pdf.Dict{
			"F":  pdf.Text("test.txt"),
			"ID": pdf.Array{pdf.String("only_one")},
		}

		spec, err := ExtractSpecification(pdf.CursorAt(x, nil), dict, true)
		if err != nil {
			t.Fatal(err)
		}

		// Should handle incomplete ID array gracefully
		if spec.ID != nil {
			t.Error("incomplete ID array should be ignored")
		}
	})
}

// TestSpecificationString verifies that a file specification string is read
// like a file specification dictionary with only an F entry, and that the
// result survives a round trip.
func TestSpecificationString(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.String("dir/a.pdf")); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		obj  pdf.Object
		want string
	}{
		{"direct", pdf.String("dir/a.pdf"), "dir/a.pdf"},
		{"indirect", ref, "dir/a.pdf"},
		{"escaped separator", pdf.String(`in\/out`), `in\/out`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := pdf.CursorAt(pdf.NewExtractor(w), nil)
			spec, err := c.Decode(tc.obj, ExtractSpecification)
			if err != nil {
				t.Fatal(err)
			}
			_, isRef := tc.obj.(pdf.Reference)
			fromDict, err := ExtractSpecification(c, pdf.Dict{"F": pdf.String(tc.want)}, !isRef)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(fromDict, spec); diff != "" {
				t.Errorf("string and dictionary differ (-dict +string):\n%s", diff)
			}

			roundTripTest(t, pdf.V1_7, spec)
		})
	}

	t.Run("empty", func(t *testing.T) {
		c := pdf.CursorAt(pdf.NewExtractor(w), nil)
		_, err := ExtractSpecification(c, pdf.String(""), true)
		if !pdf.IsMalformed(err) {
			t.Errorf("empty string: got %v, want malformed error", err)
		}
	})
}

// TestSpecificationFileNameRepair verifies how a missing F entry is
// repaired on read: it is taken from UF, it may stay absent if DOS, Mac or
// Unix is present, and a specification without any name is malformed.
func TestSpecificationFileNameRepair(t *testing.T) {
	cases := []struct {
		name string
		dict pdf.Dict
		want string // expected FileName
	}{
		{"UF in PDFDocEncoding", pdf.Dict{"UF": pdf.Text("a.pdf")}, "a.pdf"},
		{"UF outside PDFDocEncoding", pdf.Dict{"UF": pdf.Text("日本.pdf")}, "日本.pdf"},
		{"empty F and UF", pdf.Dict{"F": pdf.String(""), "UF": pdf.Text("a.pdf")}, "a.pdf"},
		{"DOS only", pdf.Dict{"DOS": pdf.String("A.PDF")}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			spec, err := ExtractSpecification(pdf.CursorAt(pdf.NewExtractor(w), nil), tc.dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if spec.FileName != tc.want {
				t.Errorf("FileName = %q, want %q", spec.FileName, tc.want)
			}
			roundTripTest(t, pdf.V1_7, spec)
		})
	}

	for name, dict := range map[string]pdf.Dict{
		"no entries": {},
		"empty F":    {"F": pdf.String("")},
		"invalid F":  {"F": pdf.Integer(1)},
	} {
		w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
		_, err := ExtractSpecification(pdf.CursorAt(pdf.NewExtractor(w), nil), dict, true)
		if !pdf.IsMalformed(err) {
			t.Errorf("%s: got %v, want malformed error", name, err)
		}
	}
}

// TestSpecificationEmbeddedWithoutName verifies that a specification whose
// only content is an embedded file is written with an empty F entry, and
// reads back unchanged.
func TestSpecificationEmbeddedWithoutName(t *testing.T) {
	spec := &Specification{
		AFRelationship: RelationshipUnspecified,
		EmbeddedFiles: map[string]*Stream{
			"F": {
				WriteData: func(w io.Writer) error {
					_, err := w.Write([]byte("content"))
					return err
				},
			},
		},
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	obj, err := rm.Embed(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	dict, err := c.Dict(obj)
	if err != nil {
		t.Fatal(err)
	}
	if f, ok := dict["F"].(pdf.String); !ok || len(f) != 0 {
		t.Errorf("F = %v, want empty string", dict["F"])
	}

	got, err := c.Decode(obj, ExtractSpecification)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(spec, got, cmp.Comparer(func(a, b *Stream) bool {
		return a.Equal(b)
	})); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestCanBeAFErrors verifies that CanBeAF reports write-side errors, never
// malformed-file errors.
func TestCanBeAFErrors(t *testing.T) {
	named := &Specification{FileName: "a.txt"}
	if err := named.CanBeAF(pdf.V1_7); !pdf.IsWrongVersion(err) {
		t.Errorf("version 1.7: got %v, want version error", err)
	}

	cases := map[string]*Specification{
		"no file name":      {},
		"unicode name only": {FileNameUnicode: pdf.Text("a.txt")},
		"only nil stream":   {EmbeddedFiles: map[string]*Stream{"F": nil}},
		"no MIME type":      {FileName: "a.txt", EmbeddedFiles: map[string]*Stream{"F": {}}},
	}
	for name, spec := range cases {
		err := spec.CanBeAF(pdf.V2_0)
		if err == nil || pdf.IsMalformed(err) {
			t.Errorf("%s: got %v, want a write-side error", name, err)
		}
	}
}

// TestCanBeAFAgreesWithEmbed verifies that CanBeAF and Embed agree on
// whether a specification has a usable file name, and that CanBeAF accepts
// the specifications which can be read back from a file.
func TestCanBeAFAgreesWithEmbed(t *testing.T) {
	stream := func(mime string) *Stream {
		return &Stream{
			MimeType:  mime,
			WriteData: func(w io.Writer) error { return nil },
		}
	}
	cases := []struct {
		name string
		spec *Specification
		ok   bool
	}{
		{"file name", &Specification{FileName: "a.txt"}, true},
		{"embedded file only", &Specification{
			EmbeddedFiles: map[string]*Stream{"F": stream("text/plain")},
		}, true},
		{"embedded file without ModDate", &Specification{
			FileName:      "a.txt",
			EmbeddedFiles: map[string]*Stream{"F": stream("text/plain")},
		}, true},
		{"no name", &Specification{}, false},
		{"unicode name only", &Specification{FileNameUnicode: pdf.Text("a.txt")}, false},
		{"nil stream only", &Specification{EmbeddedFiles: map[string]*Stream{"F": nil}}, false},
		{"empty embedded files", &Specification{EmbeddedFiles: map[string]*Stream{}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			_, embedErr := rm.Embed(tc.spec)
			afErr := tc.spec.CanBeAF(pdf.V2_0)
			if (embedErr == nil) != tc.ok || (afErr == nil) != tc.ok {
				t.Errorf("Embed: %v, CanBeAF: %v, want ok=%v", embedErr, afErr, tc.ok)
			}
		})
	}
}

// errGetter is a pdf.Getter which fails for one reference with an error which
// is not a malformed-file error.
type errGetter struct {
	pdf.Getter
	fail pdf.Reference
	err  error
}

func (g errGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.fail {
		return nil, g.err
	}
	return g.Getter.Get(ref, canObjStm)
}

// TestSpecificationOptionalEntries verifies that invalid ID and EP entries
// are dropped on read, while the rest of the specification is kept.
func TestSpecificationOptionalEntries(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	dangling := pdf.NewReference(99, 0)

	cases := []struct {
		name   string
		extra  pdf.Dict
		wantID []string
		wantEP bool
	}{
		{"valid ID", pdf.Dict{"ID": pdf.Array{pdf.String("a"), pdf.String("b")}}, []string{"a", "b"}, false},
		{"integer ID elements", pdf.Dict{"ID": pdf.Array{pdf.Integer(1), pdf.Integer(2)}}, nil, false},
		{"one bad ID element", pdf.Dict{"ID": pdf.Array{pdf.String("a"), pdf.Integer(2)}}, nil, false},
		{"dangling ID elements", pdf.Dict{"ID": pdf.Array{dangling, dangling}}, nil, false},
		{"short ID", pdf.Dict{"ID": pdf.Array{pdf.String("a")}}, nil, false},
		{"wrong type ID", pdf.Dict{"ID": pdf.Integer(1)}, nil, false},
		{"malformed EP", pdf.Dict{"EP": pdf.Integer(1)}, nil, false},
		{"EP without fields", pdf.Dict{"EP": pdf.Dict{}}, nil, false},
		{"dangling EP", pdf.Dict{"EP": dangling}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dict := pdf.Dict{"F": pdf.String("a.txt")}
			maps.Copy(dict, tc.extra)
			c := pdf.CursorAt(pdf.NewExtractor(w), nil)
			spec, err := ExtractSpecification(c, dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if spec.FileName != "a.txt" {
				t.Errorf("FileName = %q, want a.txt", spec.FileName)
			}
			if diff := cmp.Diff(tc.wantID, spec.ID); diff != "" {
				t.Errorf("ID differs (-want +got):\n%s", diff)
			}
			if (spec.EncryptedPayload != nil) != tc.wantEP {
				t.Errorf("EncryptedPayload = %v", spec.EncryptedPayload)
			}
			roundTripTest(t, pdf.V2_0, spec)
		})
	}
}

// TestSpecificationReadErrors verifies that read errors which are not
// malformed-file errors are propagated from all entries.
func TestSpecificationReadErrors(t *testing.T) {
	failRef := pdf.NewReference(7, 0)
	errTest := errors.New("test read error")
	g := errGetter{Getter: mock.Getter, fail: failRef, err: errTest}

	cases := map[string]pdf.Dict{
		"ID first":  {"ID": pdf.Array{failRef, pdf.String("b")}},
		"ID second": {"ID": pdf.Array{pdf.String("a"), failRef}},
		"EP":        {"EP": failRef},
		"RF array": {
			"EF": pdf.Dict{},
			"RF": pdf.Dict{"F": failRef},
		},
		"RF name": {
			"EF": pdf.Dict{},
			"RF": pdf.Dict{"F": pdf.Array{failRef, pdf.Dict{}}},
		},
		"RF stream": {
			"EF": pdf.Dict{},
			"RF": pdf.Dict{"F": pdf.Array{pdf.String("x"), failRef}},
		},
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			dict := pdf.Dict{"F": pdf.String("a.txt")}
			maps.Copy(dict, extra)
			c := pdf.NewCursor(g)
			_, err := ExtractSpecification(c, dict, true)
			if !errors.Is(err, errTest) {
				t.Errorf("got %v, want the read error", err)
			}
		})
	}
}

// TestSpecificationNilEmbeddedFile verifies that a nil embedded file stream
// does not count as an embedded file when checking for a file name.
func TestSpecificationNilEmbeddedFile(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	_, err := rm.Embed(&Specification{EmbeddedFiles: map[string]*Stream{"F": nil}})
	if err == nil {
		t.Error("expected error for a specification without file name")
	}
}

func roundTripTest(t *testing.T, v pdf.Version, spec1 *Specification) {
	buf, _ := memfile.NewPDFWriter(t, v, nil)
	rm := pdf.NewResourceManager(buf)

	// encode the specification
	obj, err := rm.Embed(spec1)
	if pdf.IsWrongVersion(err) {
		t.Skip()
	} else if err != nil {
		t.Fatal(err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// read back
	x := pdf.NewExtractor(buf)
	spec2, err := pdf.CursorAt(x, nil).Decode(obj, ExtractSpecification)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(spec1, spec2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func FuzzSpecificationRoundTrip(f *testing.F) {
	// Seed the fuzzer with valid test cases from all specification types
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(f, tc.version, opt)
		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(tc.spec)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}
		w.GetMeta().Trailer["Quir:E"] = embedded
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing specification")
		}
		x := pdf.NewExtractor(r)
		specification, err := pdf.CursorAt(x, nil).Decode(obj, ExtractSpecification)
		if err != nil {
			t.Skip("broken specification")
		}

		// Skip if any stream data cannot be read (e.g. unsupported filter)
		if specification.Thumbnail != nil {
			if _, err := specification.Thumbnail.Source.Pixels(); err != nil {
				t.Skip("thumbnail data not readable")
			}
		}
		for _, stream := range specification.EmbeddedFiles {
			if stream != nil && stream.WriteData != nil {
				if err := stream.WriteData(io.Discard); err != nil {
					t.Skip("embedded file data not readable")
				}
			}
		}
		for _, files := range specification.RelatedFiles {
			for _, rf := range files {
				if rf.Stream != nil && rf.Stream.WriteData != nil {
					if err := rf.Stream.WriteData(io.Discard); err != nil {
						t.Skip("related file data not readable")
					}
				}
			}
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		// Make sure we can write the specification, and read it back.
		roundTripTest(t, version, specification)
	})
}
