# Content Stream Operator Validation Design

## Overview

The `graphics/operator` package provides structured representation of PDF content stream operators with version-aware validation.

## Goals

Validate content stream operators based on:
- Operator name existence (known vs unknown operators)
- PDF version availability (operator introduced in which version)
- Deprecation status (operators deprecated in newer PDF versions)

## Design

### Package Structure

```go
package operator

import (
    "errors"
    "seehuhn.de/go/pdf"
)

// Sentinel errors for validation failures
var (
    ErrUnknown     = errors.New("unknown operator")
    ErrVersion     = errors.New("operator not available in PDF version")
    ErrDeprecated  = errors.New("deprecated operator")
)

// opInfo contains metadata about a content stream operator
type opInfo struct {
    Since      pdf.Version  // PDF version when introduced
    Deprecated pdf.Version  // PDF version when deprecated (0 if not deprecated)
}

// Operator represents a content stream operator with its arguments
type Operator struct {
    Name pdf.Name
    Args []pdf.Native
}
```

### Operator Metadata

A static map stores metadata for all 73 content stream operators, organized by category:

```go
var operators = map[pdf.Name]*opInfo{
    // General Graphics State (11 operators)
    "q":  {Since: pdf.V1_0},
    "Q":  {Since: pdf.V1_0},
    "cm": {Since: pdf.V1_0},
    "w":  {Since: pdf.V1_0},
    "J":  {Since: pdf.V1_0},
    "j":  {Since: pdf.V1_0},
    "M":  {Since: pdf.V1_0},
    "d":  {Since: pdf.V1_0},
    "ri": {Since: pdf.V1_1},
    "i":  {Since: pdf.V1_0},
    "gs": {Since: pdf.V1_2},

    // Path Construction (7 operators)
    "m":  {Since: pdf.V1_0},
    "l":  {Since: pdf.V1_0},
    "c":  {Since: pdf.V1_0},
    "v":  {Since: pdf.V1_0},
    "y":  {Since: pdf.V1_0},
    "h":  {Since: pdf.V1_0},
    "re": {Since: pdf.V1_0},

    // Path Painting (10 operators)
    "S":  {Since: pdf.V1_0},
    "s":  {Since: pdf.V1_0},
    "f":  {Since: pdf.V1_0},
    "F":  {Since: pdf.V1_0, Deprecated: pdf.V2_0},
    "f*": {Since: pdf.V1_0},
    "B":  {Since: pdf.V1_0},
    "B*": {Since: pdf.V1_0},
    "b":  {Since: pdf.V1_0},
    "b*": {Since: pdf.V1_0},
    "n":  {Since: pdf.V1_0},

    // Clipping Paths (2 operators)
    "W":  {Since: pdf.V1_0},
    "W*": {Since: pdf.V1_0},

    // Text Objects (2 operators)
    "BT": {Since: pdf.V1_0},
    "ET": {Since: pdf.V1_0},

    // Text State (7 operators)
    "Tc": {Since: pdf.V1_0},
    "Tw": {Since: pdf.V1_0},
    "Tz": {Since: pdf.V1_0},
    "TL": {Since: pdf.V1_0},
    "Tf": {Since: pdf.V1_0},
    "Tr": {Since: pdf.V1_0},
    "Ts": {Since: pdf.V1_0},

    // Text Positioning (4 operators)
    "Td": {Since: pdf.V1_0},
    "TD": {Since: pdf.V1_0},
    "Tm": {Since: pdf.V1_0},
    "T*": {Since: pdf.V1_0},

    // Text Showing (4 operators)
    "Tj": {Since: pdf.V1_0},
    "TJ": {Since: pdf.V1_0},
    "'":  {Since: pdf.V1_0},
    "\"": {Since: pdf.V1_0},

    // Type 3 Fonts (2 operators)
    "d0": {Since: pdf.V1_0},
    "d1": {Since: pdf.V1_0},

    // Colour (12 operators)
    "CS":  {Since: pdf.V1_1},
    "cs":  {Since: pdf.V1_1},
    "SC":  {Since: pdf.V1_1},
    "SCN": {Since: pdf.V1_2},
    "sc":  {Since: pdf.V1_1},
    "scn": {Since: pdf.V1_2},
    "G":   {Since: pdf.V1_0},
    "g":   {Since: pdf.V1_0},
    "RG":  {Since: pdf.V1_0},
    "rg":  {Since: pdf.V1_0},
    "K":   {Since: pdf.V1_0},
    "k":   {Since: pdf.V1_0},

    // Shading Patterns (1 operator)
    "sh": {Since: pdf.V1_3},

    // Inline Images (3 operators)
    "BI": {Since: pdf.V1_0},
    "ID": {Since: pdf.V1_0},
    "EI": {Since: pdf.V1_0},

    // XObjects (1 operator)
    "Do": {Since: pdf.V1_0},

    // Marked Content (5 operators)
    "MP":  {Since: pdf.V1_2},
    "DP":  {Since: pdf.V1_2},
    "BMC": {Since: pdf.V1_2},
    "BDC": {Since: pdf.V1_2},
    "EMC": {Since: pdf.V1_2},

    // Compatibility (2 operators)
    "BX": {Since: pdf.V1_1},
    "EX": {Since: pdf.V1_1},
}
```

Deprecated operators are included in their respective categories, not separated.

### Validation Logic

```go
func (o Operator) IsValidName(v pdf.Version) error {
    info, ok := operators[o.Name]
    if !ok {
        return ErrUnknown
    }

    if info.Deprecated != 0 && v >= info.Deprecated {
        return ErrDeprecated
    }

    if v < info.Since {
        return ErrVersion
    }

    return nil
}
```

Validation order:
1. Check operator exists (return `ErrUnknown`)
2. Check not deprecated in this version (return `ErrDeprecated`)
3. Check version is new enough (return `ErrVersion`)
4. Return nil if valid

The deprecation check precedes the version check so deprecated operators in newer PDFs return `ErrDeprecated` rather than `ErrVersion`.

### Error Handling

Three sentinel errors allow callers to distinguish failure modes using `errors.Is()`:

- `ErrUnknown`: Operator name is not a valid PDF operator
- `ErrVersion`: Operator exists but was not available in the specified PDF version
- `ErrDeprecated`: Operator is deprecated in the specified PDF version

Errors return only the sentinel value, with no additional context.

## Testing

Test cases cover:
- Known operators in valid versions (expect nil)
- Operators that are too new for the version (expect `ErrVersion`)
- Unknown operator names (expect `ErrUnknown`)
- Deprecated operators in newer versions (expect `ErrDeprecated`)
- Deprecated operators in older versions (expect nil)

Example:
```go
func TestIsValidName(t *testing.T) {
    tests := []struct{
        name    string
        op      pdf.Name
        version pdf.Version
        wantErr error
    }{
        {"known operator in valid version", "q", pdf.V1_0, nil},
        {"operator too new", "sh", pdf.V1_0, ErrVersion},
        {"unknown operator", "xyz", pdf.V2_0, ErrUnknown},
        {"deprecated operator", "F", pdf.V2_0, ErrDeprecated},
        {"deprecated operator in old version", "F", pdf.V1_7, nil},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            op := Operator{Name: tt.op}
            err := op.IsValidName(tt.version)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("got %v, want %v", err, tt.wantErr)
            }
        })
    }
}
```

## Future Extensions

The `opInfo` struct can be extended with additional fields if needed:
- Argument count or types for argument validation
- Category labels for debugging
- Replacement suggestions for deprecated operators

The static map approach makes adding new metadata straightforward.

## References

- ISO 32000-2:2020 (PDF 2.0 specification)
- `docs/content-stream-operators.md` (operator reference)
