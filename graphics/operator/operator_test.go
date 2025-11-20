package operator

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name    string
		op      pdf.Name
		version pdf.Version
		wantErr error
	}{
		// known operators in valid versions
		{"q in PDF 1.0", "q", pdf.V1_0, nil},
		{"Q in PDF 1.7", "Q", pdf.V1_7, nil},
		{"sh in PDF 1.3", "sh", pdf.V1_3, nil},
		{"gs in PDF 1.2", "gs", pdf.V1_2, nil},
		{"ri in PDF 1.1", "ri", pdf.V1_1, nil},

		// operators too new for version
		{"sh in PDF 1.0", "sh", pdf.V1_0, ErrVersion},
		{"sh in PDF 1.2", "sh", pdf.V1_2, ErrVersion},
		{"gs in PDF 1.0", "gs", pdf.V1_0, ErrVersion},
		{"gs in PDF 1.1", "gs", pdf.V1_1, ErrVersion},
		{"ri in PDF 1.0", "ri", pdf.V1_0, ErrVersion},
		{"SCN in PDF 1.1", "SCN", pdf.V1_1, ErrVersion},
		{"scn in PDF 1.1", "scn", pdf.V1_1, ErrVersion},
		{"MP in PDF 1.0", "MP", pdf.V1_0, ErrVersion},
		{"BX in PDF 1.0", "BX", pdf.V1_0, ErrVersion},

		// unknown operators
		{"xyz in PDF 1.0", "xyz", pdf.V1_0, ErrUnknown},
		{"xyz in PDF 2.0", "xyz", pdf.V2_0, ErrUnknown},
		{"foo in PDF 1.7", "foo", pdf.V1_7, ErrUnknown},

		// deprecated operators
		{"F in PDF 2.0", "F", pdf.V2_0, ErrDeprecated},

		// deprecated operators in old versions (still valid)
		{"F in PDF 1.0", "F", pdf.V1_0, nil},
		{"F in PDF 1.7", "F", pdf.V1_7, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := Operator{Name: tt.op}
			err := op.IsValidName(tt.version)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("IsValidName() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAllOperators(t *testing.T) {
	// verify we have all 73 operators
	if len(operators) != 73 {
		t.Errorf("expected 73 operators, got %d", len(operators))
	}

	// verify all operators are valid in PDF 2.0
	for name, info := range operators {
		if info.Deprecated != 0 {
			continue
		}
		op := Operator{Name: name}
		if err := op.IsValidName(pdf.V2_0); err != nil {
			t.Errorf("operator %s should be valid in PDF 2.0, got error: %v", name, err)
		}
	}
}

func TestOperatorCategories(t *testing.T) {
	// spot check operators from each category
	categories := map[string][]pdf.Name{
		"graphics state":    {"q", "Q", "cm", "w", "gs"},
		"path construction": {"m", "l", "c", "v", "h", "re"},
		"path painting":     {"S", "f", "B", "n"},
		"clipping":          {"W", "W*"},
		"text objects":      {"BT", "ET"},
		"text state":        {"Tc", "Tw", "Tf"},
		"text positioning":  {"Td", "TD", "Tm", "T*"},
		"text showing":      {"Tj", "TJ", "'", "\""},
		"type 3 fonts":      {"d0", "d1"},
		"colour":            {"CS", "cs", "SC", "G", "g", "RG", "rg", "K", "k"},
		"shading":           {"sh"},
		"inline images":     {"BI", "ID", "EI"},
		"xobjects":          {"Do"},
		"marked content":    {"MP", "DP", "BMC", "BDC", "EMC"},
		"compatibility":     {"BX", "EX"},
	}

	for category, ops := range categories {
		for _, name := range ops {
			if _, exists := operators[name]; !exists {
				t.Errorf("operator %s from category %s not found in operators map", name, category)
			}
		}
	}
}
