package builder

import "seehuhn.de/go/pdf/graphics/content"

// CompatibilityBegin begins a compatibility section.
// Unknown operators within this section are ignored.
//
// This implements the PDF graphics operator "BX".
func (b *Builder) CompatibilityBegin() {
	b.emit(content.OpBeginCompatibility)
}

// CompatibilityEnd ends a compatibility section.
//
// This implements the PDF graphics operator "EX".
func (b *Builder) CompatibilityEnd() {
	b.emit(content.OpEndCompatibility)
}
