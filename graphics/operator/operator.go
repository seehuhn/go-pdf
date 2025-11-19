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
	Since      pdf.Version // PDF version when introduced
	Deprecated pdf.Version // PDF version when deprecated (0 if not deprecated)
}

// operators maps operator names to their metadata
var operators = map[pdf.Name]*opInfo{
	// General Graphics State
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

	// Path Construction
	"m":  {Since: pdf.V1_0},
	"l":  {Since: pdf.V1_0},
	"c":  {Since: pdf.V1_0},
	"v":  {Since: pdf.V1_0},
	"y":  {Since: pdf.V1_0},
	"h":  {Since: pdf.V1_0},
	"re": {Since: pdf.V1_0},

	// Path Painting
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

	// Clipping Paths
	"W":  {Since: pdf.V1_0},
	"W*": {Since: pdf.V1_0},

	// Text Objects
	"BT": {Since: pdf.V1_0},
	"ET": {Since: pdf.V1_0},

	// Text State
	"Tc": {Since: pdf.V1_0},
	"Tw": {Since: pdf.V1_0},
	"Tz": {Since: pdf.V1_0},
	"TL": {Since: pdf.V1_0},
	"Tf": {Since: pdf.V1_0},
	"Tr": {Since: pdf.V1_0},
	"Ts": {Since: pdf.V1_0},

	// Text Positioning
	"Td": {Since: pdf.V1_0},
	"TD": {Since: pdf.V1_0},
	"Tm": {Since: pdf.V1_0},
	"T*": {Since: pdf.V1_0},

	// Text Showing
	"Tj": {Since: pdf.V1_0},
	"TJ": {Since: pdf.V1_0},
	"'":  {Since: pdf.V1_0},
	"\"": {Since: pdf.V1_0},

	// Type 3 Fonts
	"d0": {Since: pdf.V1_0},
	"d1": {Since: pdf.V1_0},

	// Colour
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

	// Shading Patterns
	"sh": {Since: pdf.V1_3},

	// Inline Images
	"BI": {Since: pdf.V1_0},
	"ID": {Since: pdf.V1_0},
	"EI": {Since: pdf.V1_0},

	// XObjects
	"Do": {Since: pdf.V1_0},

	// Marked Content
	"MP":  {Since: pdf.V1_2},
	"DP":  {Since: pdf.V1_2},
	"BMC": {Since: pdf.V1_2},
	"BDC": {Since: pdf.V1_2},
	"EMC": {Since: pdf.V1_2},

	// Compatibility
	"BX": {Since: pdf.V1_1},
	"EX": {Since: pdf.V1_1},
}

// Operator represents a content stream operator with its arguments
type Operator struct {
	Name pdf.Name
	Args []pdf.Native
}

// IsValidName checks whether the operator name is valid for the given PDF version.
// It returns ErrUnknown if the operator is not recognized, ErrDeprecated if the
// operator is deprecated in the given version, or ErrVersion if the operator was
// not yet available in the given version.
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
