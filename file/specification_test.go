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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/collection"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/image/thumbnail"
	"seehuhn.de/go/pdf/internal/debug/memfile"
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
		name:    "file specification with description",
		version: pdf.V1_6,
		spec: &Specification{
			FileName:       "document.pdf",
			Description:    "Important document",
			Volatile:       true,
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with unicode name",
		version: pdf.V1_7,
		spec: &Specification{
			FileName:        "example.txt",
			FileNameUnicode: "example_unicode.txt",
			Description:     "Unicode filename test",
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
					{Name: "related1.txt", Stream: &Stream{
						WriteData: func(w io.Writer) error {
							_, err := w.Write([]byte("Related file 1 content"))
							return err
						},
					}},
					{Name: "related2.txt", Stream: &Stream{
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
					"Title":   {Val: "Test Collection Item"},
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
				WriteData: func(w io.Writer) error {
					_, err := w.Write([]byte{0x00, 0x80, 0x80, 0xff})
					return err
				},
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
			FileNameUnicode: "https://example.com/document_unicode.pdf",
			Description:     "Comprehensive test document",
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
					{Name: "metadata.xml", Stream: &Stream{
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
					"Document": {Val: "Comprehensive Test Document"},
					"Size":     {Val: int64(1024), Prefix: "Size: "},
					"Rating":   {Val: float64(4.8)},
				},
			},
			Thumbnail: &thumbnail.Thumbnail{
				Width:            1,
				Height:           1,
				ColorSpace:       color.SpaceDeviceRGB,
				BitsPerComponent: 16,
				WriteData: func(w io.Writer) error {
					_, err := w.Write([]byte{0x00, 0x00, 0xff, 0xff, 0x00, 0x00})
					return err
				},
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
			w, _ := memfile.NewPDFWriter(tc.version, nil)
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
			decoded, err := ExtractSpecification(x, obj)
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
			Description: "No filenames",
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
		rm := pdf.NewResourceManager(buf)

		_, err := rm.Embed(spec)
		if err == nil {
			t.Error("expected error for missing file names")
		}
	})

	t.Run("version requirement - UF", func(t *testing.T) {
		spec := &Specification{
			FileName:        "test.txt",
			FileNameUnicode: "test_unicode.txt",
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_6, nil)
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

		buf, _ := memfile.NewPDFWriter(pdf.V1_2, nil)
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
				WriteData: func(w io.Writer) error {
					_, err := w.Write([]byte{0x00})
					return err
				},
			},
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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

		buf, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
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
				"F": {{Name: "related.txt", Stream: &Stream{
					WriteData: func(w io.Writer) error {
						_, err := w.Write([]byte("Related file content"))
						return err
					},
				}}},
			},
			SingleUse: true, // Try to get a direct dictionary
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
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

		buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
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

		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := rm.Embed(spec)
		if err != nil {
			t.Fatal(err)
		}

		dict, err := pdf.GetDict(buf, obj)
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

		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := rm.Embed(spec)
		if err != nil {
			t.Fatal(err)
		}

		dict, err := pdf.GetDict(buf, obj)
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
		buf, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
		x := pdf.NewExtractor(buf)

		// Create malformed dictionary with invalid RF structure
		dict := pdf.Dict{
			"Type": pdf.Name("Filespec"),
			"F":    pdf.TextString("test.txt"),
			"RF": pdf.Dict{
				"F": pdf.String("not_an_array"), // Should be array
			},
		}

		// Should handle malformed RF gracefully
		_, err := ExtractSpecification(x, dict)
		if err != nil {
			t.Errorf("should handle malformed RF gracefully: %v", err)
		}
	})

	t.Run("malformed ID array", func(t *testing.T) {
		buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
		x := pdf.NewExtractor(buf)

		// Create dictionary with malformed ID (only one element)
		dict := pdf.Dict{
			"F":  pdf.TextString("test.txt"),
			"ID": pdf.Array{pdf.String("only_one")},
		}

		spec, err := ExtractSpecification(x, dict)
		if err != nil {
			t.Fatal(err)
		}

		// Should handle incomplete ID array gracefully
		if spec.ID != nil {
			t.Error("incomplete ID array should be ignored")
		}
	})
}

func roundTripTest(t *testing.T, v pdf.Version, spec1 *Specification) {
	buf, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(buf)

	// encode the specification
	obj, err := rm.Embed(spec1)
	var versionError *pdf.VersionError
	if errors.As(err, &versionError) {
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
	spec2, err := ExtractSpecification(x, obj)
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
		w, buf := memfile.NewPDFWriter(tc.version, opt)
		rm := pdf.NewResourceManager(w)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

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
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing specification")
		}
		x := pdf.NewExtractor(r)
		specification, err := ExtractSpecification(x, obj)
		if err != nil {
			t.Skip("broken specification")
		}

		// Skip if any stream data cannot be read (e.g. unsupported filter)
		if specification.Thumbnail != nil {
			if err := specification.Thumbnail.WriteData(io.Discard); err != nil {
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

		// Make sure we can write the specification, and read it back.
		roundTripTest(t, pdf.GetVersion(r), specification)
	})
}
