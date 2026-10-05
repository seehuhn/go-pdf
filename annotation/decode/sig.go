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

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
)

// PDF 2.0 sections: 12.7.5.5

// sigFieldLock reads a signature field lock dictionary from a PDF file.
func sigFieldLock(c pdf.Cursor, obj pdf.Object, _ bool) (*acroform.SigFieldLock, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing signature field lock dictionary")
	}

	lock := &acroform.SigFieldLock{}

	// Action and, for Include / Exclude, Fields are required; there is no
	// repair which would not invent the set of locked fields, so a lock
	// without them is dropped
	action, err := pdf.Optional(c.Name(dict["Action"]))
	if err != nil {
		return nil, err
	}
	switch a := acroform.SigFieldLockAction(action); a {
	case acroform.SigFieldLockAll:
		lock.Action = a
	case acroform.SigFieldLockInclude, acroform.SigFieldLockExclude:
		lock.Action = a
		fieldsObj, err := pdf.Optional(c.Resolve(dict["Fields"]))
		if err != nil {
			return nil, err
		}
		if _, ok := fieldsObj.(pdf.Array); !ok {
			return nil, pdf.Error("missing signature field lock Fields entry")
		}
		if lock.Fields, err = readTextStringArray(c, fieldsObj); err != nil {
			return nil, err
		}
	default:
		return nil, pdf.Error("invalid signature field lock Action entry")
	}

	if p, err := pdf.Optional(c.Integer(dict["P"])); err != nil {
		return nil, err
	} else if p >= 1 && p <= 3 {
		lock.P = int(p)
	}

	return lock, nil
}

// sigSeedValue reads a seed value dictionary from a PDF file.
func sigSeedValue(c pdf.Cursor, obj pdf.Object, _ bool) (*acroform.SigSeedValue, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing seed value dictionary")
	}

	sv := &acroform.SigSeedValue{}

	if ff, err := pdf.Optional(c.Integer(dict["Ff"])); err != nil {
		return nil, err
	} else {
		sv.Flags = acroform.SigSeedValueFlags(uint32(ff))
	}

	if filter, err := pdf.Optional(c.Name(dict["Filter"])); err != nil {
		return nil, err
	} else {
		sv.Filter = filter
	}
	if sv.SubFilter, err = readNameArray(c, dict["SubFilter"]); err != nil {
		return nil, err
	}
	if sv.DigestMethod, err = readNameArray(c, dict["DigestMethod"]); err != nil {
		return nil, err
	}

	if v, err := pdf.Optional(c.Integer(dict["V"])); err != nil {
		return nil, err
	} else if v >= 1 && v <= 3 {
		sv.V = int(v)
	}

	if cert, err := pdf.DecodeOptional(c, dict["Cert"], sigCertSeedValue); err != nil {
		return nil, err
	} else {
		sv.Cert = cert
	}

	if sv.Reasons, err = readTextStringArray(c, dict["Reasons"]); err != nil {
		return nil, err
	}

	if mdp, err := pdf.Optional(c.Dict(dict["MDP"])); err != nil {
		return nil, err
	} else if mdp != nil && mdp["P"] != nil {
		// an MDP dict without /P defines no rules, so /P must stay unset
		if p, err := pdf.Optional(c.Integer(mdp["P"])); err != nil {
			return nil, err
		} else if p >= 0 && p <= 3 {
			sv.MDP.Set(uint(p))
		}
	}

	if ts, err := pdf.Optional(c.Dict(dict["TimeStamp"])); err != nil {
		return nil, err
	} else if ts != nil {
		// a timestamp dictionary exists to name a server, and there is no
		// server to fall back on, so one without a URL is dropped
		url, err := pdf.Optional(c.String(ts["URL"]))
		if err != nil {
			return nil, err
		}
		if len(url) > 0 {
			stamp := &acroform.SigSeedValueTimeStamp{URL: string(url)}
			if ff, err := pdf.Optional(c.Integer(ts["Ff"])); err != nil {
				return nil, err
			} else {
				stamp.Required = ff == 1
			}
			sv.TimeStamp = stamp
		}
	}

	if sv.LegalAttestation, err = readTextStringArray(c, dict["LegalAttestation"]); err != nil {
		return nil, err
	}

	if rev, err := pdf.Optional(c.Boolean(dict["AddRevInfo"])); err != nil {
		return nil, err
	} else {
		sv.AddRevInfo = bool(rev)
	}

	if ld, err := pdf.Optional(c.Name(dict["LockDocument"])); err != nil {
		return nil, err
	} else if ld == "true" || ld == "false" || ld == "auto" {
		sv.LockDocument = ld
	}

	if af, err := pdf.Optional(c.TextString(dict["AppearanceFilter"])); err != nil {
		return nil, err
	} else {
		sv.AppearanceFilter = string(af)
	}

	return sv, nil
}

// sigCertSeedValue reads a certificate seed value dictionary from a PDF file.
func sigCertSeedValue(c pdf.Cursor, obj pdf.Object, isDirect bool) (*acroform.SigCertSeedValue, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing certificate seed value dictionary")
	}

	cert := &acroform.SigCertSeedValue{SingleUse: isDirect}

	if ff, err := pdf.Optional(c.Integer(dict["Ff"])); err != nil {
		return nil, err
	} else {
		cert.Flags = acroform.SigCertSeedValueFlags(uint32(ff))
	}

	if cert.Subject, err = readByteStringArray(c, dict["Subject"]); err != nil {
		return nil, err
	}
	if cert.Issuer, err = readByteStringArray(c, dict["Issuer"]); err != nil {
		return nil, err
	}
	if cert.OID, err = readByteStringArray(c, dict["OID"]); err != nil {
		return nil, err
	}
	if cert.SubjectDN, err = readSubjectDN(c, dict["SubjectDN"]); err != nil {
		return nil, err
	}
	if cert.KeyUsage, err = readASCIIStringArray(c, dict["KeyUsage"]); err != nil {
		return nil, err
	}

	if url, err := pdf.Optional(c.String(dict["URL"])); err != nil {
		return nil, err
	} else {
		cert.URL = string(url)
	}
	if urlType, err := pdf.Optional(c.Name(dict["URLType"])); err != nil {
		return nil, err
	} else {
		cert.URLType = urlType
	}

	if oid, err := pdf.Optional(c.String(dict["SignaturePolicyOID"])); err != nil {
		return nil, err
	} else {
		cert.SignaturePolicyOID = string(oid)
	}
	if hash, err := pdf.Optional(c.String(dict["SignaturePolicyHashValue"])); err != nil {
		return nil, err
	} else if len(hash) > 0 {
		cert.SignaturePolicyHashValue = []byte(hash)
	}
	if alg, err := pdf.Optional(c.Name(dict["SignaturePolicyHashAlgorithm"])); err != nil {
		return nil, err
	} else {
		cert.SignaturePolicyHashAlgorithm = alg
	}
	if cert.SignaturePolicyCommitmentType, err = readASCIIStringArray(c, dict["SignaturePolicyCommitmentType"]); err != nil {
		return nil, err
	}

	// without a policy OID the other policy entries are to be ignored
	if cert.SignaturePolicyOID == "" {
		cert.SignaturePolicyHashValue = nil
		cert.SignaturePolicyHashAlgorithm = ""
		cert.SignaturePolicyCommitmentType = nil
	}

	return cert, nil
}

// readSubjectDN reads the /SubjectDN array of attribute dictionaries.
// Attributes whose value is not a text string are dropped, as are dictionaries
// left without attributes.
func readSubjectDN(c pdf.Cursor, obj pdf.Object) ([]map[pdf.Name]string, error) {
	return readArray(c, obj, func(el pdf.Object) (map[pdf.Name]string, error) {
		d, err := c.Dict(el)
		if err != nil {
			return nil, err
		}
		attrs := make(map[pdf.Name]string, len(d))
		for k, v := range d {
			s, err := c.TextString(v)
			if pdf.IsMalformed(err) {
				continue
			} else if err != nil {
				return nil, err
			}
			attrs[k] = string(s)
		}
		if len(attrs) == 0 {
			return nil, pdf.Error("empty SubjectDN attribute dictionary")
		}
		return attrs, nil
	})
}

// readArray reads an array whose elements are decoded by decode.  Elements
// which are malformed are skipped.  The result is nil if obj is absent or not
// an array, or if no element survives.
func readArray[T any](c pdf.Cursor, obj pdf.Object, decode func(pdf.Object) (T, error)) ([]T, error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return nil, err
	}
	var out []T
	for _, el := range arr {
		v, err := decode(el)
		if pdf.IsMalformed(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// readNameArray reads an array of names, skipping malformed and empty names.
func readNameArray(c pdf.Cursor, obj pdf.Object) ([]pdf.Name, error) {
	return readArray(c, obj, func(el pdf.Object) (pdf.Name, error) {
		name, err := c.Name(el)
		if err == nil && name == "" {
			err = pdf.Error("empty name")
		}
		return name, err
	})
}

// readTextStringArray reads an array of text strings.
func readTextStringArray(c pdf.Cursor, obj pdf.Object) ([]string, error) {
	return readArray(c, obj, func(el pdf.Object) (string, error) {
		s, err := c.TextString(el)
		return string(s), err
	})
}

// readASCIIStringArray reads an array of ASCII strings.
func readASCIIStringArray(c pdf.Cursor, obj pdf.Object) ([]string, error) {
	return readArray(c, obj, func(el pdf.Object) (string, error) {
		s, err := c.String(el)
		return string(s), err
	})
}

// readByteStringArray reads an array of byte strings.
func readByteStringArray(c pdf.Cursor, obj pdf.Object) ([][]byte, error) {
	return readArray(c, obj, func(el pdf.Object) ([]byte, error) {
		s, err := c.String(el)
		return []byte(s), err
	})
}
