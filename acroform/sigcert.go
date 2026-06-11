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

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 12.7.5.5

// SigCertSeedValueFlags marks which entries of a certificate seed value
// dictionary are required constraints rather than recommendations. A set bit
// makes the corresponding entry a required constraint.
type SigCertSeedValueFlags uint32

const (
	// SigCertSubject makes the Subject entry a required constraint.
	SigCertSubject SigCertSeedValueFlags = 1 << 0

	// SigCertIssuer makes the Issuer entry a required constraint.
	SigCertIssuer SigCertSeedValueFlags = 1 << 1

	// SigCertOID makes the OID entry a required constraint.
	SigCertOID SigCertSeedValueFlags = 1 << 2

	// SigCertSubjectDN makes the SubjectDN entry a required constraint.
	SigCertSubjectDN SigCertSeedValueFlags = 1 << 3

	// SigCertKeyUsage makes the KeyUsage entry a required constraint.
	SigCertKeyUsage SigCertSeedValueFlags = 1 << 5

	// SigCertURL makes the URL entry a required constraint.
	SigCertURL SigCertSeedValueFlags = 1 << 6
)

// SigCertSeedValue is a certificate seed value dictionary. It constrains the
// characteristics of the certificate used when signing a signature field.
//
// It corresponds to the /Cert entry of a seed value dictionary.
type SigCertSeedValue struct {
	// Flags marks which entries are required constraints.
	//
	// This corresponds to the /Ff entry.
	Flags SigCertSeedValueFlags

	// Subject holds DER-encoded X.509v3 certificates that are acceptable for
	// signing.
	Subject [][]byte

	// Issuer holds DER-encoded X.509v3 certificates of acceptable issuers.
	Issuer [][]byte

	// OID holds object identifiers of certificate policies that shall be present
	// in the signing certificate.
	OID [][]byte

	// SubjectDN holds subject distinguished names that shall be present in the
	// signing certificate. Each entry maps certificate attribute identifiers to
	// their required values.
	//
	// This entry requires PDF 1.7.
	SubjectDN []map[pdf.Name]string

	// KeyUsage holds acceptable key-usage extension patterns. Each string
	// encodes the required state of the key-usage bits using the characters
	// '0', '1', and 'X'.
	//
	// This entry requires PDF 1.7.
	KeyUsage []string

	// URL (optional) is a URL whose use is defined by URLType.
	URL string

	// URLType (optional) names the usage of URL. An empty value indicates that
	// no usage is specified.
	//
	// This entry requires PDF 1.7.
	URLType pdf.Name

	// SignaturePolicyOID (optional) is the OID of the signature policy to use
	// when signing.
	//
	// This entry requires PDF 2.0.
	SignaturePolicyOID string

	// SignaturePolicyHashValue (optional) is the hash value of the signature
	// policy.
	//
	// This entry requires PDF 2.0.
	SignaturePolicyHashValue []byte

	// SignaturePolicyHashAlgorithm (optional) is the hash function used to
	// compute SignaturePolicyHashValue.
	//
	// This entry requires PDF 2.0.
	SignaturePolicyHashAlgorithm pdf.Name

	// SignaturePolicyCommitmentType (optional) lists the commitment types that
	// may be used within the signature policy.
	//
	// This entry requires PDF 2.0.
	SignaturePolicyCommitmentType []string

	// SingleUse determines whether Embed returns a dictionary (true) or a
	// reference (false).
	SingleUse bool
}

var _ pdf.Embedder = (*SigCertSeedValue)(nil)

// Embed writes the certificate seed value dictionary to the PDF file.
//
// This implements the [pdf.Embedder] interface.
func (c *SigCertSeedValue) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "certificate seed value dictionary", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("SVCert")
	}

	if c.Flags != 0 {
		dict["Ff"] = pdf.Integer(uint32(c.Flags))
	}

	writeByteStringArray(dict, "Subject", c.Subject)
	writeByteStringArray(dict, "Issuer", c.Issuer)
	writeByteStringArray(dict, "OID", c.OID)

	if len(c.SubjectDN) > 0 {
		if err := pdf.CheckVersion(e.Out(), "certificate seed value SubjectDN entry", pdf.V1_7); err != nil {
			return nil, err
		}
		dict["SubjectDN"] = writeSubjectDN(c.SubjectDN)
	}
	if len(c.KeyUsage) > 0 {
		if err := pdf.CheckVersion(e.Out(), "certificate seed value KeyUsage entry", pdf.V1_7); err != nil {
			return nil, err
		}
		writeASCIIStringArray(dict, "KeyUsage", c.KeyUsage)
	}

	if c.URL != "" {
		dict["URL"] = pdf.String(c.URL)
	}
	if c.URLType != "" {
		if err := pdf.CheckVersion(e.Out(), "certificate seed value URLType entry", pdf.V1_7); err != nil {
			return nil, err
		}
		dict["URLType"] = c.URLType
	}

	if c.SignaturePolicyOID != "" {
		if err := pdf.CheckVersion(e.Out(), "certificate seed value SignaturePolicyOID entry", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["SignaturePolicyOID"] = pdf.String(c.SignaturePolicyOID)
	}
	if len(c.SignaturePolicyHashValue) > 0 {
		if err := pdf.CheckVersion(e.Out(), "certificate seed value SignaturePolicyHashValue entry", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["SignaturePolicyHashValue"] = pdf.String(c.SignaturePolicyHashValue)
	}
	if c.SignaturePolicyHashAlgorithm != "" {
		if err := pdf.CheckVersion(e.Out(), "certificate seed value SignaturePolicyHashAlgorithm entry", pdf.V2_0); err != nil {
			return nil, err
		}
		dict["SignaturePolicyHashAlgorithm"] = c.SignaturePolicyHashAlgorithm
	}
	if len(c.SignaturePolicyCommitmentType) > 0 {
		if err := pdf.CheckVersion(e.Out(), "certificate seed value SignaturePolicyCommitmentType entry", pdf.V2_0); err != nil {
			return nil, err
		}
		writeASCIIStringArray(dict, "SignaturePolicyCommitmentType", c.SignaturePolicyCommitmentType)
	}

	if c.SingleUse {
		return dict, nil
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// writeSubjectDN encodes the /SubjectDN array of attribute dictionaries.
func writeSubjectDN(dns []map[pdf.Name]string) pdf.Array {
	arr := make(pdf.Array, len(dns))
	for i, attrs := range dns {
		d := pdf.Dict{}
		for k, v := range attrs {
			d[k] = pdf.TextString(v)
		}
		arr[i] = d
	}
	return arr
}
