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
	FileNameUnicode string

	// FileName specifies the file path in platform-independent format.
	// Components are separated by forward slashes.
	//
	// The characters in this string must be in the PDFDocEncoding character
	// set.
	//
	// This corresponds to the /F entry in the PDF file specification
	// dictionary.
	FileName string

	// FileNameDOS (optional, deprecated in PDF 2.0) specifies a DOS file name.
	//
	// This corresponds to the /DOS entry in the PDF dictionary.
	FileNameDOS string

	// FileNameMac (optional, deprecated in PDF 2.0) specifies a MacOS
	// file name.
	//
	// This corresponds to the /Mac entry in the PDF dictionary.
	FileNameMac string

	// FileNameUnix (optional, deprecated in PDF 2.0) specifies a Unix
	// file name.
	//
	// This corresponds to the /Unix entry in the PDF dictionary.
	FileNameUnix string

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
	Description string

	// Thumbnail (PDF 2.0) references a thumbnail image stream for the file.
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

// ExtractSpecification extracts a file specification dictionary from a PDF object.
func ExtractSpecification(x *pdf.Extractor, obj pdf.Object) (*Specification, error) {
	dict, err := pdf.GetDictTyped(x.R, obj, "Filespec")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Errorf("missing file specification dictionary")
	}

	spec := &Specification{}

	// NameSpace (FS)
	if nameSpace, err := pdf.Optional(pdf.GetName(x.R, dict["FS"])); err != nil {
		return nil, err
	} else {
		spec.NameSpace = nameSpace
	}

	// File names
	if fileName, err := pdf.Optional(pdf.GetTextString(x.R, dict["F"])); err != nil {
		return nil, err
	} else {
		spec.FileName = string(fileName)
	}

	if fileNameUnicode, err := pdf.Optional(pdf.GetTextString(x.R, dict["UF"])); err != nil {
		return nil, err
	} else {
		spec.FileNameUnicode = string(fileNameUnicode)
	}

	if fileNameDOS, err := pdf.Optional(pdf.GetString(x.R, dict["DOS"])); err != nil {
		return nil, err
	} else {
		spec.FileNameDOS = string(fileNameDOS)
	}

	if fileNameMac, err := pdf.Optional(pdf.GetString(x.R, dict["Mac"])); err != nil {
		return nil, err
	} else {
		spec.FileNameMac = string(fileNameMac)
	}

	if fileNameUnix, err := pdf.Optional(pdf.GetString(x.R, dict["Unix"])); err != nil {
		return nil, err
	} else {
		spec.FileNameUnix = string(fileNameUnix)
	}

	// Description
	if description, err := pdf.Optional(pdf.GetTextString(x.R, dict["Desc"])); err != nil {
		return nil, err
	} else {
		spec.Description = string(description)
	}

	// ID array
	if idArray, err := pdf.Optional(pdf.GetArray(x.R, dict["ID"])); err != nil {
		return nil, err
	} else if len(idArray) >= 2 {
		id1, err1 := pdf.Optional(pdf.GetString(x.R, idArray[0]))
		id2, err2 := pdf.Optional(pdf.GetString(x.R, idArray[1]))
		if err1 == nil && err2 == nil {
			spec.ID = []string{string(id1), string(id2)}
		}
	}

	// Volatile
	if volatile, err := pdf.Optional(pdf.GetBoolean(x.R, dict["V"])); err != nil {
		return nil, err
	} else {
		spec.Volatile = bool(volatile)
	}

	// EmbeddedFiles (EF)
	if efDict, err := pdf.Optional(pdf.GetDict(x.R, dict["EF"])); err != nil {
		return nil, err
	} else if efDict != nil {
		spec.EmbeddedFiles = make(map[string]*Stream)
		for key, value := range efDict {
			if stream, err := pdf.ExtractorGetOptional(x, value, ExtractStream); err != nil {
				return nil, err
			} else if stream != nil {
				spec.EmbeddedFiles[string(key)] = stream
			}
		}
	}

	// RelatedFiles (RF)
	if rfDict, err := pdf.Optional(pdf.GetDict(x.R, dict["RF"])); err != nil {
		return nil, err
	} else if rfDict != nil {
		spec.RelatedFiles, err = extractRelatedFiles(x, rfDict)
		if err != nil {
			return nil, err
		}
	}

	// AFRelationship
	if afRelationship, err := pdf.Optional(pdf.GetName(x.R, dict["AFRelationship"])); err != nil {
		return nil, err
	} else if afRelationship != "" {
		spec.AFRelationship = Relationship(afRelationship)
	} else {
		spec.AFRelationship = RelationshipUnspecified
	}

	// CollectionItem (CI)
	if ci, err := pdf.ExtractorGetOptional(x, dict["CI"], collection.ExtractItemDict); err != nil {
		return nil, err
	} else {
		spec.CollectionItem = ci
	}

	// Thumbnail
	if thumb, err := pdf.ExtractorGetOptional(x, dict["Thumb"], thumbnail.ExtractThumbnail); err != nil {
		return nil, err
	} else {
		spec.Thumbnail = thumb
	}

	// EncryptedPayload (EP)
	if epObj := dict["EP"]; epObj != nil {
		ep, err := ExtractEncryptedPayload(x, epObj)
		if err != nil {
			return nil, err
		}
		spec.EncryptedPayload = ep
	}

	return spec, nil
}

// Embed converts the file specification to a PDF dictionary.
func (spec *Specification) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	// Check version requirements for various fields
	if spec.FileNameUnicode != "" || spec.CollectionItem != nil {
		if err := pdf.CheckVersion(rm.Out(), "file specification UF/CI entries", pdf.V1_7); err != nil {
			return nil, zero, err
		}
	}

	if spec.EmbeddedFiles != nil || spec.RelatedFiles != nil {
		if err := pdf.CheckVersion(rm.Out(), "file specification EF/RF entries", pdf.V1_3); err != nil {
			return nil, zero, err
		}
	}

	if spec.Thumbnail != nil || spec.EncryptedPayload != nil || (spec.AFRelationship != "" && spec.AFRelationship != RelationshipUnspecified) {
		if err := pdf.CheckVersion(rm.Out(), "file specification PDF 2.0 entries", pdf.V2_0); err != nil {
			return nil, zero, err
		}
	}

	// Validate that F is present if DOS/Mac/Unix are all absent
	if spec.FileName == "" && spec.FileNameDOS == "" && spec.FileNameMac == "" && spec.FileNameUnix == "" {
		return nil, zero, pdf.Errorf("file specification must have F entry if DOS, Mac, and Unix entries are all absent")
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
	if spec.FileName != "" {
		dict["F"] = pdf.TextString(spec.FileName)
	}

	if spec.FileNameUnicode != "" {
		dict["UF"] = pdf.TextString(spec.FileNameUnicode)
	}

	if spec.FileNameDOS != "" {
		dict["DOS"] = pdf.String(spec.FileNameDOS)
	}

	if spec.FileNameMac != "" {
		dict["Mac"] = pdf.String(spec.FileNameMac)
	}

	if spec.FileNameUnix != "" {
		dict["Unix"] = pdf.String(spec.FileNameUnix)
	}

	// Description
	if spec.Description != "" {
		dict["Desc"] = pdf.TextString(spec.Description)
	}

	// ID array
	if len(spec.ID) >= 2 {
		dict["ID"] = pdf.Array{pdf.String(spec.ID[0]), pdf.String(spec.ID[1])}
	}

	// Volatile
	if spec.Volatile {
		dict["V"] = pdf.Boolean(spec.Volatile)
	}

	// EmbeddedFiles (EF)
	if len(spec.EmbeddedFiles) > 0 {
		efDict := pdf.Dict{}
		for key, stream := range spec.EmbeddedFiles {
			if stream != nil {
				ref, _, err := pdf.EmbedHelperEmbed(rm, stream)
				if err != nil {
					return nil, zero, err
				}
				efDict[pdf.Name(key)] = ref
			}
		}
		dict["EF"] = efDict
	}

	// RelatedFiles (RF)
	if len(spec.RelatedFiles) > 0 {
		rfDict, err := encodeRelatedFiles(rm, spec.RelatedFiles)
		if err != nil {
			return nil, zero, err
		}
		dict["RF"] = rfDict
	}

	// AFRelationship
	if spec.AFRelationship != "" && spec.AFRelationship != RelationshipUnspecified {
		dict["AFRelationship"] = pdf.Name(spec.AFRelationship)
	}

	// CollectionItem (CI)
	if spec.CollectionItem != nil {
		ci, _, err := pdf.EmbedHelperEmbed(rm, spec.CollectionItem)
		if err != nil {
			return nil, zero, err
		}
		dict["CI"] = ci
	}

	// Thumbnail
	if spec.Thumbnail != nil {
		thumb, _, err := pdf.EmbedHelperEmbed(rm, spec.Thumbnail)
		if err != nil {
			return nil, zero, err
		}
		dict["Thumb"] = thumb
	}

	// EncryptedPayload (EP)
	if spec.EncryptedPayload != nil {
		ep, _, err := pdf.EmbedHelperEmbed(rm, spec.EncryptedPayload)
		if err != nil {
			return nil, zero, err
		}
		dict["EP"] = ep
	}

	// If EF or RF is present, the file specification dictionary must be indirectly referenced
	mustBeIndirect := spec.EmbeddedFiles != nil || spec.RelatedFiles != nil

	if mustBeIndirect && spec.SingleUse {
		return nil, zero, pdf.Errorf("file specification with EF or RF entries must be indirect (SingleUse must be false)")
	}

	if spec.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

// RelatedFile represents an entry in a related files array.
type RelatedFile struct {
	Name   string
	Stream *Stream
}

// extractRelatedFiles extracts related files from an RF dictionary.
func extractRelatedFiles(x *pdf.Extractor, rfDict pdf.Dict) (map[string][]RelatedFile, error) {
	result := make(map[string][]RelatedFile)

	for key, value := range rfDict {
		array, err := pdf.GetArray(x.R, value)
		if err != nil {
			continue // skip malformed entries
		}

		var relatedFiles []RelatedFile
		for i := 0; i < len(array); i += 2 {
			if i+1 >= len(array) {
				break
			}

			name, err := pdf.Optional(pdf.GetTextString(x.R, array[i]))
			if err != nil {
				continue
			}

			if stream, err := pdf.ExtractorGetOptional(x, array[i+1], ExtractStream); err != nil {
				continue // skip malformed stream
			} else if stream != nil {
				relatedFiles = append(relatedFiles, RelatedFile{
					Name:   string(name),
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

	for key, files := range relatedFiles {
		var array pdf.Array
		for _, file := range files {
			array = append(array, pdf.TextString(file.Name))
			if file.Stream != nil {
				ref, _, err := pdf.EmbedHelperEmbed(rm, file.Stream)
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
// for the given PDF version. Returns an error if requirements are not met.
func (spec *Specification) CanBeAF(v pdf.Version) error {
	// Associated files require PDF 2.0
	if v < pdf.V2_0 {
		return pdf.Errorf("associated files require PDF 2.0 or later")
	}

	// Must have at least one file name
	if spec.FileName == "" && spec.FileNameUnicode == "" &&
		spec.FileNameDOS == "" && spec.FileNameMac == "" && spec.FileNameUnix == "" {
		return pdf.Errorf("file specification must have at least one file name")
	}

	// Check embedded file requirements (per spec 14.13.2)
	for key, stream := range spec.EmbeddedFiles {
		if stream == nil {
			continue
		}

		// Subtype (MIME type) is required for associated files
		if stream.MimeType == "" {
			return pdf.Errorf("embedded file %q must have Subtype (MIME type) for associated files", key)
		}

		// ModDate is required in Params for associated files
		if stream.ModDate.IsZero() {
			return pdf.Errorf("embedded file %q must have ModDate in Params for associated files", key)
		}
	}

	return nil
}
