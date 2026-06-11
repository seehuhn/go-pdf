// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package acroform

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 12.7.5.5

// SigSeedValueFlags marks which entries of a seed value dictionary are required
// constraints rather than recommendations. A set bit makes the corresponding
// entry a required constraint.
type SigSeedValueFlags uint32

const (
	// SigSeedFilter makes the Filter entry a required constraint.
	SigSeedFilter SigSeedValueFlags = 1 << 0

	// SigSeedSubFilter makes the SubFilter entry a required constraint.
	SigSeedSubFilter SigSeedValueFlags = 1 << 1

	// SigSeedV makes the V entry a required constraint.
	SigSeedV SigSeedValueFlags = 1 << 2

	// SigSeedReasons makes the Reasons entry a required constraint.
	SigSeedReasons SigSeedValueFlags = 1 << 3

	// SigSeedLegalAttestation makes the LegalAttestation entry a required
	// constraint.
	SigSeedLegalAttestation SigSeedValueFlags = 1 << 4

	// SigSeedAddRevInfo makes the AddRevInfo entry a required constraint.
	SigSeedAddRevInfo SigSeedValueFlags = 1 << 5

	// SigSeedDigestMethod makes the DigestMethod entry a required constraint.
	SigSeedDigestMethod SigSeedValueFlags = 1 << 6

	// SigSeedLockDocument makes the LockDocument entry a required constraint.
	SigSeedLockDocument SigSeedValueFlags = 1 << 7

	// SigSeedAppearanceFilter makes the AppearanceFilter entry a required
	// constraint.
	SigSeedAppearanceFilter SigSeedValueFlags = 1 << 8
)

// SigSeedValueTimeStamp specifies a timestamp server for signing.
type SigSeedValueTimeStamp struct {
	// URL is the URL of an RFC 3161 timestamping server. It is required.
	URL string

	// Required indicates that the signature must carry a timestamp.
	Required bool
}

// SigSeedValue is a signature field seed value dictionary. It constrains the
// properties of a signature applied to the containing field.
//
// It corresponds to the /SV entry of a signature field and is always written as
// an indirect object.
type SigSeedValue struct {
	// Flags marks which entries are required constraints.
	//
	// This corresponds to the /Ff entry.
	Flags SigSeedValueFlags

	// Filter (optional) is the signature handler to use when signing.
	Filter pdf.Name

	// SubFilter (optional) lists acceptable signature encodings, in order of
	// preference.
	SubFilter []pdf.Name

	// DigestMethod (optional) lists acceptable digest algorithms.
	//
	// This entry requires PDF 1.7.
	DigestMethod []pdf.Name

	// V specifies the minimum seed value parser capability required, as one of
	// the values 1, 2, or 3. The value 0 indicates that no requirement is set.
	//
	// This corresponds to the /V entry.
	V int

	// Cert (optional) constrains the certificate used for signing.
	Cert *SigCertSeedValue

	// Reasons (optional) lists permitted reasons for signing.
	Reasons []string

	// MDP holds the modification-detection permission level, as a value from 0
	// to 3.
	//
	// This entry requires PDF 1.6.
	MDP optional.UInt

	// TimeStamp (optional) specifies a timestamp server.
	//
	// This entry requires PDF 1.6.
	TimeStamp *SigSeedValueTimeStamp

	// LegalAttestation (optional) lists permitted legal attestations.
	//
	// This entry requires PDF 1.6.
	LegalAttestation []string

	// AddRevInfo indicates that revocation information shall be embedded in the
	// signature.
	//
	// This entry requires PDF 1.7.
	AddRevInfo bool

	// LockDocument (optional) records the author's intent for locking the
	// document at signing time. Permitted values are "true", "false", and
	// "auto". An empty value indicates that no intent is recorded.
	//
	// This entry requires PDF 2.0.
	LockDocument pdf.Name

	// AppearanceFilter (optional) names the appearance to use when signing.
	//
	// This entry requires PDF 2.0.
	AppearanceFilter string
}

var _ pdf.Embedder = (*SigSeedValue)(nil)

// Embed writes the seed value dictionary to the PDF file and returns a reference
// to it.
//
// This implements the [pdf.Embedder] interface.
func (s *SigSeedValue) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "seed value dictionary", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("SV")
	}

	if s.Flags != 0 {
		dict["Ff"] = pdf.Integer(uint32(s.Flags))
	}

	if s.Filter != "" {
		dict["Filter"] = s.Filter
	}
	writeNameArray(dict, "SubFilter", s.SubFilter)

	if len(s.DigestMethod) > 0 {
		if err := pdf.CheckVersion(e.Out(), "seed value DigestMethod entry", pdf.V1_7); err != nil {
			return nil, err
		}
		writeNameArray(dict, "DigestMethod", s.DigestMethod)
	}

	if s.V != 0 {
		if s.V < 1 || s.V > 3 {
			return nil, fmt.Errorf("invalid seed value version %d", s.V)
		}
		dict["V"] = pdf.Integer(s.V)
	}

	if s.Cert != nil {
		cert, err := e.Embed(s.Cert)
		if err != nil {
			return nil, err
		}
		dict["Cert"] = cert
	}

	writeTextStringArray(dict, "Reasons", s.Reasons)

	if p, ok := s.MDP.Get(); ok {
		if p > 3 {
			return nil, fmt.Errorf("invalid seed value MDP permission %d", p)
		}
		if err := pdf.CheckVersion(e.Out(), "seed value MDP entry", pdf.V1_6); err != nil {
			return nil, err
		}
		dict["MDP"] = pdf.Dict{"P": pdf.Integer(p)}
	}

	if s.TimeStamp != nil {
		if s.TimeStamp.URL == "" {
			return nil, errors.New("seed value TimeStamp requires a URL")
		}
		if err := pdf.CheckVersion(e.Out(), "seed value TimeStamp entry", pdf.V1_6); err != nil {
			return nil, err
		}
		ts := pdf.Dict{"URL": pdf.String(s.TimeStamp.URL)}
		if s.TimeStamp.Required {
			ts["Ff"] = pdf.Integer(1)
		}
		dict["TimeStamp"] = ts
	}

	if len(s.LegalAttestation) > 0 {
		if err := pdf.CheckVersion(e.Out(), "seed value LegalAttestation entry", pdf.V1_6); err != nil {
			return nil, err
		}
		writeTextStringArray(dict, "LegalAttestation", s.LegalAttestation)
	}

	if s.AddRevInfo {
		if err := pdf.CheckVersion(e.Out(), "seed value AddRevInfo entry", pdf.V1_7); err != nil {
			return nil, err
		}
		dict["AddRevInfo"] = pdf.Boolean(true)
	}

	if s.LockDocument != "" {
		if s.LockDocument != "true" && s.LockDocument != "false" && s.LockDocument != "auto" {
			return nil, fmt.Errorf("invalid seed value LockDocument %q", s.LockDocument)
		}
		if err := pdf.CheckVersion(e.Out(), "seed value LockDocument entry", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["LockDocument"] = s.LockDocument
	}

	if s.AppearanceFilter != "" {
		if err := pdf.CheckVersion(e.Out(), "seed value AppearanceFilter entry", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["AppearanceFilter"] = pdf.TextString(s.AppearanceFilter)
	}

	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
