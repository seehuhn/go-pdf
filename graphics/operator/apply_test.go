package operator

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/resource"
)

func TestApplyOperator_BasicStructure(t *testing.T) {
	state := &State{}
	op := Operator{Name: "q", Args: nil}
	res := &resource.Resource{}

	err := ApplyOperator(state, op, res)
	if err != nil {
		t.Errorf("ApplyOperator returned error for valid operator: %v", err)
	}
}

func TestArgParser_GetFloat(t *testing.T) {
	tests := []struct {
		name    string
		args    []pdf.Native
		want    float64
		wantErr bool
	}{
		{"Real", []pdf.Native{pdf.Real(3.14)}, 3.14, false},
		{"Integer", []pdf.Native{pdf.Integer(42)}, 42.0, false},
		{"WrongType", []pdf.Native{pdf.Name("foo")}, 0, true},
		{"NoArgs", []pdf.Native{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := argParser{args: tt.args}
			got := p.GetFloat()
			if got != tt.want {
				t.Errorf("GetFloat() = %v, want %v", got, tt.want)
			}
			if (p.err != nil) != tt.wantErr {
				t.Errorf("GetFloat() error = %v, wantErr %v", p.err, tt.wantErr)
			}
		})
	}
}

func TestArgParser_Check(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*argParser)
		wantErr bool
	}{
		{"NoArgs", func(p *argParser) {}, false},
		{"ExtraArgs", func(p *argParser) { p.args = []pdf.Native{pdf.Integer(1)} }, true},
		{"PreviousError", func(p *argParser) { p.err = errors.New("test") }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &argParser{}
			tt.setup(p)
			err := p.Check()
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
