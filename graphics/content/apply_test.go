// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package content

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestApplyOperator_BasicStructure(t *testing.T) {
	state := &GraphicsState{}
	op := Operator{Name: "q", Args: nil}
	res := &Resources{}

	err := state.Apply(res, op)
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

func TestArgParser_GetInt(t *testing.T) {
	tests := []struct {
		name    string
		args    []pdf.Native
		want    int
		wantErr bool
	}{
		{"Integer", []pdf.Native{pdf.Integer(42)}, 42, false},
		{"WrongType", []pdf.Native{pdf.Real(3.14)}, 0, true},
		{"NoArgs", []pdf.Native{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := argParser{args: tt.args}
			got := p.GetInt()
			if got != tt.want {
				t.Errorf("GetInt() = %v, want %v", got, tt.want)
			}
			if (p.err != nil) != tt.wantErr {
				t.Errorf("GetInt() error = %v, wantErr %v", p.err, tt.wantErr)
			}
		})
	}
}

func TestArgParser_GetName(t *testing.T) {
	tests := []struct {
		name    string
		args    []pdf.Native
		want    pdf.Name
		wantErr bool
	}{
		{"Name", []pdf.Native{pdf.Name("foo")}, pdf.Name("foo"), false},
		{"WrongType", []pdf.Native{pdf.Integer(42)}, pdf.Name(""), true},
		{"NoArgs", []pdf.Native{}, pdf.Name(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := argParser{args: tt.args}
			got := p.GetName()
			if got != tt.want {
				t.Errorf("GetName() = %v, want %v", got, tt.want)
			}
			if (p.err != nil) != tt.wantErr {
				t.Errorf("GetName() error = %v, wantErr %v", p.err, tt.wantErr)
			}
		})
	}
}

func TestArgParser_GetArray(t *testing.T) {
	tests := []struct {
		name    string
		args    []pdf.Native
		want    pdf.Array
		wantErr bool
	}{
		{"Array", []pdf.Native{pdf.Array{pdf.Integer(1), pdf.Integer(2)}}, pdf.Array{pdf.Integer(1), pdf.Integer(2)}, false},
		{"WrongType", []pdf.Native{pdf.Integer(42)}, nil, true},
		{"NoArgs", []pdf.Native{}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := argParser{args: tt.args}
			got := p.GetArray()
			if len(got) != len(tt.want) {
				t.Errorf("GetArray() = %v, want %v", got, tt.want)
			}
			if (p.err != nil) != tt.wantErr {
				t.Errorf("GetArray() error = %v, wantErr %v", p.err, tt.wantErr)
			}
		})
	}
}

func TestArgParser_GetDict(t *testing.T) {
	tests := []struct {
		name    string
		args    []pdf.Native
		want    pdf.Dict
		wantErr bool
	}{
		{"Dict", []pdf.Native{pdf.Dict{"Key": pdf.Integer(1)}}, pdf.Dict{"Key": pdf.Integer(1)}, false},
		{"WrongType", []pdf.Native{pdf.Integer(42)}, nil, true},
		{"NoArgs", []pdf.Native{}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := argParser{args: tt.args}
			got := p.GetDict()
			if len(got) != len(tt.want) {
				t.Errorf("GetDict() = %v, want %v", got, tt.want)
			}
			if (p.err != nil) != tt.wantErr {
				t.Errorf("GetDict() error = %v, wantErr %v", p.err, tt.wantErr)
			}
		})
	}
}

func TestArgParser_GetString(t *testing.T) {
	tests := []struct {
		name    string
		args    []pdf.Native
		want    pdf.String
		wantErr bool
	}{
		{"String", []pdf.Native{pdf.String("hello")}, pdf.String("hello"), false},
		{"WrongType", []pdf.Native{pdf.Integer(42)}, nil, true},
		{"NoArgs", []pdf.Native{}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := argParser{args: tt.args}
			got := p.GetString()
			if string(got) != string(tt.want) {
				t.Errorf("GetString() = %v, want %v", got, tt.want)
			}
			if (p.err != nil) != tt.wantErr {
				t.Errorf("GetString() error = %v, wantErr %v", p.err, tt.wantErr)
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
