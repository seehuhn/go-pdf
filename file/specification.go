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
	"errors"
	"fmt"
	"maps"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/collection"
	"seehuhn.de/go/pdf/graphics/image/thumbnail"
)

// PDF 2.0 sections: 7.11.3 7.11.4

// Specification represents a PDF file specification dictionary.
// File specifications can refer to external files or embedded files
// within the PDF document.
type Specification struct {
	// NameSpace (optional) specifies the file system to interpret this file
	// specification in. Set to "URL" if FileName is an URL.
	NameSpace pdf.Name

	// FileNameUnicode (optional, PDF 1.7) provides a Unicode version of the
	// file name. Components are separated by forward slashes.
	//
	// If FileNameUnicode is present, PDF reader should use this instead of
	// FileName.
	//
	// This corresponds to the /UF entry in the PDF dictionary.
	FileNameUnicode pdf.TextString

	// FileName (optional) specifies the file path in platform-independent
	// format.  Components are separated by forward slashes.
	// At least one of FileName, FileNameDOS, FileNameMac and FileNameUnix
	// must be set, unless EmbeddedFiles is non-empty.
	//
	// The characters in this string should be in the PDFDocEncoding
	// character set.  Other names are written in UTF-16.
	//
	// This corresponds to the /F entry in the PDF file specification
	// dictionary.
	FileName string

	// FileNameDOS (optional, deprecated in PDF 2.0) specifies a DOS file name.
	// At least one of FileName, FileNameDOS, FileNameMac and FileNameUnix
	// must be set, unless EmbeddedFiles is non-empty.
	//
	// This corresponds to the /DOS entry in the PDF dictionary.
	FileNameDOS pdf.String

	// FileNameMac (optional, deprecated in PDF 2.0) specifies a MacOS
	// file name.
	// At least one of FileName, FileNameDOS, FileNameMac and FileNameUnix
	// must be set, unless EmbeddedFiles is non-empty.
	//
	// This corresponds to the /Mac entry in the PDF dictionary.
	FileNameMac pdf.String

	// FileNameUnix (optional, deprecated in PDF 2.0) specifies a Unix
	// file name.
	// At least one of FileName, FileNameDOS, FileNameMac and FileNameUnix
	// must be set, unless EmbeddedFiles is non-empty.
	//
	// This corresponds to the /Unix entry in the PDF dictionary.
	FileNameUnix pdf.String

	// AFRelationship (PDF 2.0) specifies the relationship between the
	// referencing component and the associated file.
	//
	// When writing file specifications, and empty name can be used as a
	// shorthand for [RelationshipUnspecified].
	AFRelationship Relationship

	// Description (optional) provides descriptive text for the file
	// specification. Used for display in user interfaces.
	//
	// This corresponds to the /Desc entry in the PDF dictionary.
	Description pdf.TextString

	// Thumbnail (PDF 2.0) references a thumbnail image stream for the file.
	//
	// This corresponds to the /Thumb entry in the PDF dictionary.
	Thumbnail *thumbnail.Thumbnail

	// ID contains an identifier for the described file, as two byte strings.
	ID []string

	// Volatile indicates whether the file is volatile (changes frequently).
	// When true, applications should not cache the file.
	//
	// This corresponds to the /V entry in the PDF dictionary.
	Volatile bool

	// EmbeddedFiles maps file specification keys (FileName and/or
	// FileNameUnicode) to embedded file streams. When present, the files
	// are embedded in the PDF document.
	//
	// This corresponds to the /EF entry in the PDF dictionary.
	EmbeddedFiles map[string]*Stream

	// RelatedFiles (optional) maps file specification keys to related files
	// arrays. Each array identifies files related to those in EF. Keys must
	// match those in EmbeddedFiles.
	//
	// This corresponds to the /RF entry in the PDF dictionary.
	RelatedFiles map[string][]RelatedFile

	// EncryptedPayload (optional, PDF 2.0) references an encrypted payload
	// dictionary. Required when referencing encrypted payload documents.
	//
	// This corresponds to the /EP entry in the PDF dictionary.
	EncryptedPayload *EncryptedPayload

	// CollectionItem (optional) specifies a collection item dictionary for
	// portable collections.
	//
	// This corresponds to the /CI entry in the PDF dictionary.
	CollectionItem *collection.ItemDict

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractSpecification extracts a file specification from a PDF object.
// Both forms of file specification are accepted: a dictionary, and a string,
// which is read like a dictionary with only an F entry.
func ExtractSpecification(c pdf.Cursor, obj pdf.Object, isDirect bool) (*Specification, error) {
	resolved, err := c.Resolve(obj)
	if err != nil {
		return nil, err
	}

	var dict pdf.Dict
	if s, ok := resolved.(pdf.String); ok {
		// file specification string (PDF 7.11.1)
		if s.AsTextString().IsZero() {
			return nil, pdf.Errorf("empty file specification string")
		}
		dict = pdf.Dict{"F": s}
	} else {
		dict, err = c.DictTyped(resolved, "Filespec")
		if err != nil {
			return nil, err
		} else if dict == nil {
			return nil, pdf.Errorf("missing file specification dictionary")
		}
	}

	spec := &Specification{}

	// NameSpace (FS)
	if nameSpace, err := pdf.Optional(c.Name(dict["FS"])); err != nil {
		return nil, err
	} else {
		spec.NameSpace = nameSpace
	}

	// File names
	if fileName, err := pdf.Optional(c.TextString(dict["F"])); err != nil {
		return nil, err
	} else {
		spec.FileName = fileName.String()
	}

	if fileNameUnicode, err := pdf.Optional(c.TextString(dict["UF"])); err != nil {
		return nil, err
	} else {
		spec.FileNameUnicode = fileNameUnicode
	}

	if fileNameDOS, err := pdf.Optional(c.String(dict["DOS"])); err != nil {
		return nil, err
	} else {
		spec.FileNameDOS = fileNameDOS
	}

	if fileNameMac, err := pdf.Optional(c.String(dict["Mac"])); err != nil {
		return nil, err
	} else {
		spec.FileNameMac = fileNameMac
	}

	if fileNameUnix, err := pdf.Optional(c.String(dict["Unix"])); err != nil {
		return nil, err
	} else {
		spec.FileNameUnix = fileNameUnix
	}

	// Description
	if description, err := pdf.Optional(c.TextString(dict["Desc"])); err != nil {
		return nil, err
	} else {
		spec.Description = description
	}

	// ID array
	if idArray, err := pdf.Optional(c.Array(dict["ID"])); err != nil {
		return nil, err
	} else if len(idArray) >= 2 {
		// both elements must be strings, otherwise ID is dropped
		id1, err := pdf.Optional(c.Resolve(idArray[0]))
		if err != nil {
			return nil, err
		}
		id2, err := pdf.Optional(c.Resolve(idArray[1]))
		if err != nil {
			return nil, err
		}
		s1, ok1 := id1.(pdf.String)
		s2, ok2 := id2.(pdf.String)
		if ok1 && ok2 {
			spec.ID = []string{string(s1), string(s2)}
		}
	}

	// Volatile
	volatile, _, err := c.OptionalBoolean(dict["V"])
	if err != nil {
		return nil, err
	}
	spec.Volatile = volatile

	// EmbeddedFiles (EF)
	if efDict, err := pdf.Optional(c.Dict(dict["EF"])); err != nil {
		return nil, err
	} else if efDict != nil {
		spec.EmbeddedFiles = make(map[string]*Stream)
		for key, value := range efDict {
			if stream, err := c.DecodeOptional(value, ExtractStream); err != nil {
				return nil, err
			} else if stream != nil {
				spec.EmbeddedFiles[string(key)] = stream
			}
		}
	}

	// RelatedFiles (RF)
	// Per spec, RF requires EF to be present, so ignore RF if EF is absent.
	if rfDict, err := pdf.Optional(c.Dict(dict["RF"])); err != nil {
		return nil, err
	} else if rfDict != nil && spec.EmbeddedFiles != nil {
		spec.RelatedFiles, err = extractRelatedFiles(c, rfDict)
		if err != nil {
			return nil, err
		}
	}

	// AFRelationship
	if afRelationship, err := pdf.Optional(c.Name(dict["AFRelationship"])); err != nil {
		return nil, err
	} else if afRelationship != "" {
		spec.AFRelationship = Relationship(afRelationship)
	} else {
		spec.AFRelationship = RelationshipUnspecified
	}

	// CollectionItem (CI)
	if ci, err := c.DecodeOptional(dict["CI"], collection.ExtractItemDict); err != nil {
		return nil, err
	} else {
		spec.CollectionItem = ci
	}

	// Thumbnail
	if thumb, err := c.DecodeOptional(dict["Thumb"], thumbnail.ExtractThumbnail); err != nil {
		return nil, err
	} else {
		spec.Thumbnail = thumb
	}

	// EncryptedPayload (EP)
	if ep, err := c.DecodeOptional(dict["EP"], ExtractEncryptedPayload); err != nil {
		return nil, err
	} else {
		spec.EncryptedPayload = ep
	}

	// F is required if DOS, Mac and Unix are all absent, and UF is an
	// addition to F, not a replacement.  A missing F is taken from UF.
	// Without any name, only a specification with embedded files is still
	// usable; it is written with an empty F.
	if spec.FileName == "" && len(spec.FileNameDOS) == 0 && len(spec.FileNameMac) == 0 && len(spec.FileNameUnix) == 0 {
		if !spec.FileNameUnicode.IsZero() {
			spec.FileName = spec.FileNameUnicode.String()
		} else if len(spec.EmbeddedFiles) == 0 {
			return nil, pdf.Error("file specification without a file name")
		}
	}

	spec.SingleUse = isDirect

	return spec, nil
}

// checkNamed checks that the specification can be written with an F entry
// which a reader accepts: either a file name is given, or the specification
// consists of embedded files which stand in for it.
func (spec *Specification) checkNamed() error {
	hasName := spec.FileName != "" ||
		len(spec.FileNameDOS) > 0 || len(spec.FileNameMac) > 0 || len(spec.FileNameUnix) > 0
	if hasName {
		return nil
	}
	for _, stream := range spec.EmbeddedFiles {
		if stream != nil {
			return nil
		}
	}
	return errors.New("file specification needs FileName, FileNameDOS, FileNameMac or FileNameUnix")
}

// Embed converts the file specification to a PDF dictionary.
func (spec *Specification) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	// Check version requirements for various fields
	if !spec.FileNameUnicode.IsZero() || spec.CollectionItem != nil {
		if err := pdf.CheckVersion(rm.Out(), "file specification UF/CI entries", pdf.V1_7); err != nil {
			return nil, err
		}
	}

	if spec.EmbeddedFiles != nil || spec.RelatedFiles != nil {
		if err := pdf.CheckVersion(rm.Out(), "file specification EF/RF entries", pdf.V1_3); err != nil {
			return nil, err
		}
	}

	if spec.Thumbnail != nil || spec.EncryptedPayload != nil || (spec.AFRelationship != "" && spec.AFRelationship != RelationshipUnspecified) {
		if err := pdf.CheckVersion(rm.Out(), "file specification PDF 2.0 entries", pdf.V2_0); err != nil {
			return nil, err
		}
	}

	// F is required if DOS, Mac and Unix are all absent
	hasPlatformName := len(spec.FileNameDOS) > 0 || len(spec.FileNameMac) > 0 || len(spec.FileNameUnix) > 0
	if err := spec.checkNamed(); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}

	// Type field is required if EF, EP, or RF is present
	requiresType := spec.EmbeddedFiles != nil || spec.EncryptedPayload != nil || spec.RelatedFiles != nil
	if requiresType || rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Filespec")
	}

	// NameSpace (FS)
	if spec.NameSpace != "" {
		dict["FS"] = spec.NameSpace
	}

	// File names
	if spec.FileName != "" || !hasPlatformName {
		dict["F"] = pdf.Text(spec.FileName)
	}

	if !spec.FileNameUnicode.IsZero() {
		dict["UF"] = spec.FileNameUnicode
	}

	if len(spec.FileNameDOS) > 0 {
		dict["DOS"] = spec.FileNameDOS
	}

	if len(spec.FileNameMac) > 0 {
		dict["Mac"] = spec.FileNameMac
	}

	if len(spec.FileNameUnix) > 0 {
		dict["Unix"] = spec.FileNameUnix
	}

	// Description
	if !spec.Description.IsZero() {
		dict["Desc"] = spec.Description
	}

	// ID array
	if len(spec.ID) >= 2 {
		dict["ID"] = pdf.Array{pdf.String(spec.ID[0]), pdf.String(spec.ID[1])}
	}

	// Volatile
	if spec.Volatile {
		dict["V"] = pdf.Boolean(spec.Volatile)
	}

	// EmbeddedFiles (EF).  Embedding allocates object numbers, so the keys
	// are sorted to keep the output reproducible.
	if spec.EmbeddedFiles != nil {
		efDict := pdf.Dict{}
		for _, key := range slices.Sorted(maps.Keys(spec.EmbeddedFiles)) {
			stream := spec.EmbeddedFiles[key]
			if stream != nil {
				ref, err := rm.Embed(stream)
				if err != nil {
					return nil, err
				}
				efDict[pdf.Name(key)] = ref
			}
		}
		dict["EF"] = efDict
	}

	// RelatedFiles (RF)
	// Per spec, RF requires EF to be present.
	if spec.RelatedFiles != nil {
		if spec.EmbeddedFiles == nil {
			return nil, errors.New("RelatedFiles requires EmbeddedFiles to be present")
		}
		rfDict, err := encodeRelatedFiles(rm, spec.RelatedFiles)
		if err != nil {
			return nil, err
		}
		dict["RF"] = rfDict
	}

	// AFRelationship
	if spec.AFRelationship != "" && spec.AFRelationship != RelationshipUnspecified {
		dict["AFRelationship"] = pdf.Name(spec.AFRelationship)
	}

	// CollectionItem (CI)
	if spec.CollectionItem != nil {
		ci, err := rm.Embed(spec.CollectionItem)
		if err != nil {
			return nil, err
		}
		dict["CI"] = ci
	}

	// Thumbnail
	if spec.Thumbnail != nil {
		thumb, err := rm.Embed(spec.Thumbnail)
		if err != nil {
			return nil, err
		}
		dict["Thumb"] = thumb
	}

	// EncryptedPayload (EP)
	if spec.EncryptedPayload != nil {
		ep, err := rm.Embed(spec.EncryptedPayload)
		if err != nil {
			return nil, err
		}
		dict["EP"] = ep
	}

	// If EF or RF is present, the file specification dictionary must be indirectly referenced
	mustBeIndirect := spec.EmbeddedFiles != nil || spec.RelatedFiles != nil

	if mustBeIndirect && spec.SingleUse {
		return nil, errors.New("file specification with EF or RF entries must be indirect (SingleUse must be false)")
	}

	if spec.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// RelatedFile represents an entry in a related files array.
type RelatedFile struct {
	Name   pdf.TextString
	Stream *Stream
}

// extractRelatedFiles extracts related files from an RF dictionary.
func extractRelatedFiles(c pdf.Cursor, rfDict pdf.Dict) (map[string][]RelatedFile, error) {
	result := make(map[string][]RelatedFile)

	for key, value := range rfDict {
		array, err := pdf.Optional(c.Array(value))
		if err != nil {
			return nil, err
		}

		var relatedFiles []RelatedFile
		for i := 0; i < len(array); i += 2 {
			if i+1 >= len(array) {
				break
			}

			name, err := pdf.Optional(c.TextString(array[i]))
			if err != nil {
				return nil, err
			}

			if stream, err := c.DecodeOptional(array[i+1], ExtractStream); err != nil {
				return nil, err
			} else if stream != nil {
				relatedFiles = append(relatedFiles, RelatedFile{
					Name:   name,
					Stream: stream,
				})
			}
		}

		if len(relatedFiles) > 0 {
			result[string(key)] = relatedFiles
		}
	}

	return result, nil
}

// encodeRelatedFiles creates an RF dictionary from related files map.
func encodeRelatedFiles(rm *pdf.EmbedHelper, relatedFiles map[string][]RelatedFile) (pdf.Dict, error) {
	rfDict := pdf.Dict{}

	for _, key := range slices.Sorted(maps.Keys(relatedFiles)) {
		files := relatedFiles[key]
		var array pdf.Array
		for _, file := range files {
			array = append(array, file.Name)
			if file.Stream != nil {
				ref, err := rm.Embed(file.Stream)
				if err != nil {
					return nil, err
				}
				array = append(array, ref)
			}
		}
		rfDict[pdf.Name(key)] = array
	}

	return rfDict, nil
}

// Relationship represents the relationship between a file specification
// and the referencing component.
type Relationship pdf.Name

// These are the standard relationship types defined in PDF 2.0.
const (
	RelationshipSource           Relationship = "Source"
	RelationshipData             Relationship = "Data"
	RelationshipAlternative      Relationship = "Alternative"
	RelationshipSupplement       Relationship = "Supplement"
	RelationshipEncryptedPayload Relationship = "EncryptedPayload"
	RelationshipFormData         Relationship = "FormData"
	RelationshipSchema           Relationship = "Schema"
	RelationshipUnspecified      Relationship = "Unspecified"
)

// CanBeAF checks if this file specification can be used as an associated file
// for the given PDF version.  It returns an error if requirements are not met.
//
// The conditions are those which [Specification.Embed] imposes on the file
// specification, plus PDF 2.0 and the requirement that every embedded file
// stream has a MIME type (PDF 2.0, section 14.13.2).  The modification date
// of an embedded file is recommended but not required.
func (spec *Specification) CanBeAF(v pdf.Version) error {
	// Associated files require PDF 2.0
	if v < pdf.V2_0 {
		return &pdf.VersionError{Operation: "associated files", Earliest: pdf.V2_0}
	}

	if err := spec.checkNamed(); err != nil {
		return err
	}

	// The keys are sorted so that a file with several faults always reports
	// the same one.
	for _, key := range slices.Sorted(maps.Keys(spec.EmbeddedFiles)) {
		stream := spec.EmbeddedFiles[key]
		if stream != nil && stream.MimeType == "" {
			return fmt.Errorf("embedded file %q must have Subtype (MIME type) for associated files", key)
		}
	}

	return nil
}
