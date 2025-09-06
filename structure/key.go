package structure

import "seehuhn.de/go/pdf"

// Key represents a key for the Parent Tree.
// This is used for StructParent and StructParents entries in PDF objects.
type Key struct {
	val uint64
}

func NewKey(v pdf.Integer) Key {
	var k Key
	k.Set(v)
	return k
}

func (k Key) Get() (pdf.Integer, bool) {
	if k.val == 0 {
		return 0, false
	}
	return pdf.Integer(k.val - 1), true
}

func (k *Key) Set(v pdf.Integer) {
	if v < 0 || uint64(v) >= 1<<64-1 {
		panic("key value out of range")
	}
	k.val = uint64(v) + 1
}

func (k *Key) Clear() {
	k.val = 0
}

// Equal compares two Keys for equality.
func (k Key) Equal(other Key) bool {
	return k.val == other.val
}
