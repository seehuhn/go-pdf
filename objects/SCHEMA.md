# PDF Object Schema Format

Machine-readable descriptions of PDF object types and their Go representations.

## File Organization

Files are organized by PDF 2.0 spec chapter:

```
objects/
  SCHEMA.md          # this file
  chapter07.yaml     # Document Structure (excluding 7.3 basic types)
  chapter08.yaml     # Graphics
  chapter09.yaml     # Text
  chapter10.yaml     # Fonts (partial - see also chapter09)
  chapter11.yaml     # Rendering
  chapter12.yaml     # Interactive Features
  chapter13.yaml     # Multimedia
  chapter14.yaml     # Document Interchange
```

## Defaults

| Field | Default | Specify when |
|-------|---------|--------------|
| `goRepr` | `struct` | interface, array, etc. |
| `pdfRepr` | `dict` | stream, array |
| `required` | `false` | field is required |
| `introduced` | `"1.0"` | later version |
| `typeRequired` | `false` | Type field is required |
| `subtypeRequired` | `true` | Subtype field is optional |
| `goPresenceTracking` | `false` | uses optional.* or pointer for presence |
| `discriminator` | `false` | field is PDF-only type discriminator |
| `embeds` | `[]` | type embeds other types (Go struct embedding) |

## File Structure

```yaml
chapter: 12
title: "Interactive Features"

types:
  - name: TextAnnotation
    # ...

enums:
  - name: TextIcon
    # ...

interfaces:
  - name: Annotation
    # ...
```

## Type Definition

```yaml
types:
  - name: TextAnnotation
    goType: "annotation.Text"
    pdfType: Annot
    pdfSubtype: Text
    pdfRepr: dict
    readFunc: "annotation.Extract"
    writeMethod: "(*Text).Encode"
    specSection: "12.5.6.4"
    specTable: "Table 176"

    fields:
      # ...

  # Stream-based type
  - name: Type0Function
    goType: "function.Type0"
    pdfType: null                         # no /Type field
    pdfRepr: stream
    readFunc: "function.Extract"
    writeMethod: "(*Type0).Embed"
    specSection: "7.10.2"
    specTable: "Table 38"

  # Unimplemented type
  - name: 3DAnnotation
    goType: null
    pdfType: Annot
    pdfSubtype: "3D"
    pdfRepr: dict
    specSection: "13.6.2"
    specTable: "Table 311"
    introduced: "1.6"

  # Embedded type (not standalone in PDF)
  - name: AnnotCommon
    goType: "annotation.Common"
    pdfType: null                         # embedded, not standalone
    pdfRepr: dict
    notes: "Fields common to all annotations"
    fields:
      # ...

  # Type with embedding
  - name: TextAnnotation
    goType: "annotation.Text"
    pdfType: Annot
    pdfSubtype: Text
    embeds: [AnnotCommon, AnnotMarkup]    # inherits fields from these types
    fields:
      # only type-specific fields here
```

## Field Definition

```yaml
fields:
  # Minimal
  - goName: Contents
    goType: string
    pdfKey: "Contents"
    pdfType: string
    purpose: "Text to display"

  # With default value
  - goName: Open
    goType: bool
    pdfKey: "Open"
    pdfType: boolean
    default: false
    purpose: "Initially open"

  # Required field
  - goName: Rect
    goType: "pdf.Rectangle"
    pdfKey: "Rect"
    pdfType: rectangle
    required: true
    purpose: "Annotation rectangle"

  # Later version
  - goName: AssociatedFiles
    goType: "[]*file.Specification"
    pdfKey: "AF"
    pdfType: array
    introduced: "2.0"
    purpose: "Associated files"

  # Presence tracking (Case 2 optional)
  - goName: StructParent
    goType: "optional.UInt"
    goPresenceTracking: true
    pdfKey: "StructParent"
    pdfType: integer
    purpose: "Structure tree key"

  # Reference to another type
  - goName: Resources
    goType: "*content.Resources"
    pdfKey: "Resources"
    pdfType: dictionary
    refType: ResourceDictionary
    purpose: "Resource dictionary"

  # Closed enum (reference)
  - goName: Icon
    goType: TextIcon
    pdfKey: "Name"
    pdfType: name
    default: "Note"
    enum: TextIcon
    purpose: "Icon to display"

  # Open enum (inline values)
  - goName: BlendMode
    goType: "pdf.Name"
    pdfKey: "BM"
    pdfType: name
    default: "Normal"
    standardValues: ["Normal", "Multiply", "Screen"]
    purpose: "Blend mode"

  # Unimplemented field
  - goName: null
    pdfKey: "Mix"
    pdfType: boolean
    purpose: "Mix with ambient sound"

  # Discriminator field (PDF-only, used for type dispatch)
  - pdfKey: "Type"
    pdfType: name
    discriminator: true
    possibleValues: ["Annot"]

  # Value transformation (Go and PDF use different conventions)
  - goName: NonStrokingTransparency
    goType: float64
    goTransform: invert                   # Go: 0=opaque, PDF: 1=opaque
    pdfKey: "ca"
    pdfType: number
    purpose: "Non-stroking transparency"
```

## Special Mappings

```yaml
fields:
  # Bitfield: multiple Go bools -> one PDF integer
  - mapping: bitfield
    goFields:
      - { name: IsFixedPitch, type: bool, bit: 0 }
      - { name: IsSerif, type: bool, bit: 1 }
      - { name: IsSymbolic, type: bool, bit: 2 }
    pdfKey: "Flags"
    pdfType: integer
    required: true
    purpose: "Font descriptor flags"

  # Array pairs: semantic pairs in flat array
  - goName: Domain
    goType: "[]float64"
    mapping: arrayPairs
    pairMeaning: [min, max]
    pdfKey: "Domain"
    pdfType: array
    required: true
    purpose: "Input ranges"

  # Composite: multiple PDF fields -> one Go field
  - goName: State
    goType: TextState
    mapping: composite
    pdfFields: ["StateModel", "State"]
    purpose: "Combined state"
```

## Enum Definition

```yaml
enums:
  # Name-based
  - name: TextIcon
    goType: "annotation.TextIcon"
    default: "Note"
    values:
      - { go: TextIconComment, pdf: "Comment" }
      - { go: TextIconKey, pdf: "Key" }
      - { go: TextIconNote, pdf: "Note" }

  # Int-based
  - name: Rotation
    goType: "page.Rotation"
    default: { go: Rotate0, pdf: 0 }
    values:
      - { go: Rotate0, goValue: 1, pdf: 0 }
      - { go: Rotate90, goValue: 2, pdf: 90 }
      - { go: Rotate180, goValue: 3, pdf: 180 }
      - { go: Rotate270, goValue: 4, pdf: 270 }
    notes: "goValue=0 means inherit"
```

## Interface Definition

```yaml
interfaces:
  - name: Annotation
    goType: "annotation.Annotation"
    extractFunc: "annotation.Extract"
    discriminatorKey: "Subtype"
    discriminatorType: name
    implementations:
      - { type: TextAnnotation, value: "Text" }
      - { type: LinkAnnotation, value: "Link" }

  - name: Function
    goType: "pdf.Function"
    extractFunc: "function.Extract"
    discriminatorKey: "FunctionType"
    discriminatorType: integer
    implementations:
      - { type: Type0Function, value: 0 }
      - { type: Type2Function, value: 2 }
```

## Common yq Queries

```bash
# All types
yq '.types[].name' objects/*.yaml

# Unimplemented types
yq '.types[] | select(.goType == null) | .name' objects/*.yaml

# Unimplemented fields
yq '.types[] | {type: .name, field: .fields[] | select(.goName == null) | .pdfKey}' objects/*.yaml

# PDF types with multiple Go representations
yq ea '[.types[]] | group_by(.pdfType) | .[] | select(length > 1) | .[].name' objects/*.yaml

# Objects containing resource dictionaries
yq ea '.types[] | select(.fields[].refType == "ResourceDictionary") | .name' objects/*.yaml

# All fields introduced after 1.0
yq '.types[].fields[] | select(.introduced != null) | {name: .goName, version: .introduced}' objects/*.yaml

# All required fields
yq '.types[] | {type: .name, fields: [.fields[] | select(.required == true) | .goName]}' objects/*.yaml
```
