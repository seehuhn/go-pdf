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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
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
			FileNameDOS:    "EXAMPLE.TXT",
			FileNameMac:    "example.txt",
			FileNameUnix:   "example.txt",
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with embedded files",
		version: pdf.V1_3,
		spec: &Specification{
			FileName: "document.pdf",
			EmbeddedFiles: map[string]pdf.Reference{
				"F":  pdf.NewReference(100, 0),
				"UF": pdf.NewReference(101, 0),
			},
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with related files",
		version: pdf.V1_3,
		spec: &Specification{
			FileName: "main.txt",
			EmbeddedFiles: map[string]pdf.Reference{
				"F": pdf.NewReference(200, 0),
			},
			RelatedFiles: map[string][]RelatedFile{
				"F": {
					{Name: "related1.txt", Stream: pdf.NewReference(201, 0)},
					{Name: "related2.txt", Stream: pdf.NewReference(202, 0)},
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
			FileName:       "collection_item.pdf",
			CollectionItem: pdf.NewReference(300, 0),
			AFRelationship: RelationshipUnspecified,
		},
	},
	{
		name:    "file specification with thumbnail",
		version: pdf.V2_0,
		spec: &Specification{
			FileName:       "image.jpg",
			Thumbnail:      pdf.NewReference(400, 0),
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
			EmbeddedFiles: map[string]pdf.Reference{
				"F":  pdf.NewReference(500, 0),
				"UF": pdf.NewReference(501, 0),
			},
			RelatedFiles: map[string][]RelatedFile{
				"F": {
					{Name: "metadata.xml", Stream: pdf.NewReference(502, 0)},
				},
			},
			AFRelationship: RelationshipSource,
			CollectionItem: pdf.NewReference(503, 0),
			Thumbnail:      pdf.NewReference(504, 0),
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

			// Encode the specification
			obj, err := tc.spec.Encode(rm)
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

			// Decode it back
			decoded, err := DecodeSpecification(w, obj)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.spec, decoded); diff != "" {
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

		_, err := spec.Encode(rm)
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

		_, err := spec.Encode(rm)
		if err == nil {
			t.Error("expected version error for UF in PDF 1.6")
		}
	})

	t.Run("version requirement - EF", func(t *testing.T) {
		spec := &Specification{
			FileName: "test.txt",
			EmbeddedFiles: map[string]pdf.Reference{
				"F": pdf.NewReference(100, 0),
			},
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_2, nil)
		rm := pdf.NewResourceManager(buf)

		_, err := spec.Encode(rm)
		if err == nil {
			t.Error("expected version error for EF in PDF 1.2")
		}
	})

	t.Run("version requirement - PDF 2.0", func(t *testing.T) {
		spec := &Specification{
			FileName:  "test.txt",
			Thumbnail: pdf.NewReference(100, 0),
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(buf)

		_, err := spec.Encode(rm)
		if err == nil {
			t.Error("expected version error for Thumbnail in PDF 1.7")
		}
	})
}

func TestSpecificationIndirectReference(t *testing.T) {
	t.Run("EF requires indirect reference", func(t *testing.T) {
		spec := &Specification{
			FileName: "test.txt",
			EmbeddedFiles: map[string]pdf.Reference{
				"F": pdf.NewReference(100, 0),
			},
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := spec.Encode(rm)
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
				"F": {{Name: "related.txt", Stream: pdf.NewReference(101, 0)}},
			},
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := spec.Encode(rm)
		if err != nil {
			t.Fatal(err)
		}

		// Should return a reference, not a direct dictionary
		if _, isRef := obj.(pdf.Reference); !isRef {
			t.Error("expected indirect reference when RF is present")
		}
	})

	t.Run("simple spec can be direct", func(t *testing.T) {
		spec := &Specification{
			FileName: "test.txt",
		}

		buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
		rm := pdf.NewResourceManager(buf)

		obj, err := spec.Encode(rm)
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

		obj, err := spec.Encode(rm)
		if err != nil {
			t.Fatal(err)
		}

		dict, ok := obj.(pdf.Dict)
		if !ok {
			t.Fatal("expected dictionary")
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

		obj, err := spec.Encode(rm)
		if err != nil {
			t.Fatal(err)
		}

		dict, ok := obj.(pdf.Dict)
		if !ok {
			t.Fatal("expected dictionary")
		}

		if dict["AFRelationship"] != pdf.Name("Source") {
			t.Error("AFRelationship should be written when not Unspecified")
		}
	})
}

func TestSpecificationMalformedInput(t *testing.T) {
	t.Run("malformed RF dictionary", func(t *testing.T) {
		buf, _ := memfile.NewPDFWriter(pdf.V1_3, nil)

		// Create malformed dictionary with invalid RF structure
		dict := pdf.Dict{
			"Type": pdf.Name("Filespec"),
			"F":    pdf.TextString("test.txt"),
			"RF": pdf.Dict{
				"F": pdf.String("not_an_array"), // Should be array
			},
		}

		// Should handle malformed RF gracefully
		_, err := DecodeSpecification(buf, dict)
		if err != nil {
			t.Errorf("should handle malformed RF gracefully: %v", err)
		}
	})

	t.Run("malformed ID array", func(t *testing.T) {
		buf, _ := memfile.NewPDFWriter(pdf.V1_0, nil)

		// Create dictionary with malformed ID (only one element)
		dict := pdf.Dict{
			"F":  pdf.TextString("test.txt"),
			"ID": pdf.Array{pdf.String("only_one")},
		}

		spec, err := DecodeSpecification(buf, dict)
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
	obj, err := spec1.Encode(rm)
	if _, isVersionError := err.(*pdf.VersionError); isVersionError {
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
	spec2, err := DecodeSpecification(buf, obj)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(spec1, spec2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func FuzzRoundTrip(f *testing.F) {
	// Seed the fuzzer with valid test cases from all specification types
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)
		rm := pdf.NewResourceManager(w)

		embedded, err := tc.spec.Encode(rm)
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
		specification, err := DecodeSpecification(r, obj)
		if err != nil {
			t.Skip("broken specification")
		}

		// Make sure we can write the specification, and read it back.
		roundTripTest(t, pdf.GetVersion(r), specification)
	})
}
