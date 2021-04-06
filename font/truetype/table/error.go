package table

// ErrNoTable indicates that a required table is missing from a TrueType or
// OpenType font file.
type ErrNoTable struct {
	Name string
}

func (err *ErrNoTable) Error() string {
	return "missing " + err.Name + " table in font"
}

// IsMissing returns true, if err indicates a missing sfnt table.
func IsMissing(err error) bool {
	_, missing := err.(*ErrNoTable)
	return missing
}
