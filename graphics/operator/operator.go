package operator

import "seehuhn.de/go/pdf"

type Operator struct {
	Name pdf.Name
	Args []pdf.Native
}

func (o Operator) IsValidName(v pdf.Version) error {
	panic("not implemented")
}
