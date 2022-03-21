package funit

// Int16 is a 16-bit integer in font design units.
type Int16 int16

// Uint16 is an unsigned 16-bit integer in font design units.
type Uint16 uint16

// Rect represents a rectangle in font design units.
type Rect struct {
	LLx, LLy, URx, URy Int16
}

// IsZero is true if the glyph leaves no marks on the page.
func (rect Rect) IsZero() bool {
	return rect.LLx == 0 && rect.LLy == 0 && rect.URx == 0 && rect.URy == 0
}

// Extend enlarges the rectangle to also cover `other`.
func (rect *Rect) Extend(other Rect) {
	if other.IsZero() {
		return
	}
	if rect.IsZero() {
		*rect = other
		return
	}
	if other.LLx < rect.LLx {
		rect.LLx = other.LLx
	}
	if other.LLy < rect.LLy {
		rect.LLy = other.LLy
	}
	if other.URx > rect.URx {
		rect.URx = other.URx
	}
	if other.URy > rect.URy {
		rect.URy = other.URy
	}
}
