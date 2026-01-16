package optional

// Bool represents an optional boolean value.
//
// This is used for PDF fields that have three states: not set, true, or false.
// An example is the Trapped field in the document information dictionary,
// where "not set" means "Unknown".
type Bool struct {
	isSet bool
	val   bool
}

// NewBool creates a new Bool with the given value.
func NewBool(v bool) Bool {
	var b Bool
	b.Set(v)
	return b
}

// Get returns the value and whether it is set.
func (b Bool) Get() (bool, bool) {
	return b.val, b.isSet
}

// Set sets the value.
func (b *Bool) Set(v bool) {
	b.isSet = true
	b.val = v
}

// Clear clears the value.
func (b *Bool) Clear() {
	b.isSet = false
	b.val = false
}

// Equal compares two Bools for equality.
func (b Bool) Equal(other Bool) bool {
	return b.isSet == other.isSet && b.val == other.val
}
