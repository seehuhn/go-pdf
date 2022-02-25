package cff

// NotSupportedError indicates that the font file seems valid but uses a
// CFF feature which is not supported by this library.
type NotSupportedError struct {
	Feature string
}

func (err *NotSupportedError) Error() string {
	return "cff: " + err.Feature + " not supported"
}

func notSupported(feature string) error {
	return &NotSupportedError{feature}
}

// InvalidFontError indicates a problem with the font file.
type InvalidFontError struct {
	Reason string
}

func (err *InvalidFontError) Error() string {
	return "cff: " + err.Reason
}

func invalidSince(reason string) error {
	return &InvalidFontError{reason}
}
