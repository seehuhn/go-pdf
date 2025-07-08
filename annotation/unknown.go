package annotation

import "seehuhn.de/go/pdf"

type Unknown struct {
	Common

	// Data contains the raw data of the annotation.
	Data pdf.Dict
}

var _ pdf.Annotation = (*Unknown)(nil)

// AnnotationType returns the subtype of the unknown annotation.
// This implements the [pdf.Annotation] interface.
func (u *Unknown) AnnotationType() string {
	if subtype, ok := u.Data["Subtype"].(pdf.Name); ok {
		return string(subtype)
	}
	return "Unknown"
}

func (u *Unknown) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// Start with the raw data
	dict := make(pdf.Dict)
	for k, v := range u.Data {
		dict[k] = v
	}

	// Ensure Type and Subtype are set
	dict["Type"] = pdf.Name("Annot")
	if dict["Subtype"] == nil {
		dict["Subtype"] = pdf.Name("Unknown")
	}

	// Add/override common annotation fields
	if err := u.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	return dict, zero, nil
}

func extractUnknown(r pdf.Getter, dict pdf.Dict) (*Unknown, error) {
	unknown := &Unknown{
		Data: dict,
	}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &unknown.Common); err != nil {
		return nil, err
	}

	return unknown, nil
}
