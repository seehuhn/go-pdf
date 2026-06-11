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
func sigFieldLock(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*acroform.SigFieldLock, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing signature field lock dictionary")
	}

	lock := &acroform.SigFieldLock{}

	// Action; snap an unrecognised value to a safe default so the result stays
	// writable
	action, err := pdf.Optional(x.GetName(path, dict["Action"]))
	if err != nil {
		return nil, err
	}
	switch acroform.SigFieldLockAction(action) {
	case acroform.SigFieldLockInclude:
		lock.Action = acroform.SigFieldLockInclude
	case acroform.SigFieldLockExclude:
		lock.Action = acroform.SigFieldLockExclude
	default:
		lock.Action = acroform.SigFieldLockAll
	}

	// Fields applies only to Include / Exclude
	if lock.Action != acroform.SigFieldLockAll {
		if fields, err := readTextStringArray(x, path, dict["Fields"]); err != nil {
			return nil, err
		} else {
			lock.Fields = fields
		}
	}

	if p, err := pdf.Optional(x.GetInteger(path, dict["P"])); err != nil {
		return nil, err
	} else if p >= 1 && p <= 3 {
		lock.P = int(p)
	}

	return lock, nil
}

// sigSeedValue reads a seed value dictionary from a PDF file.
func sigSeedValue(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*acroform.SigSeedValue, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing seed value dictionary")
	}

	sv := &acroform.SigSeedValue{}

	if ff, err := pdf.Optional(x.GetInteger(path, dict["Ff"])); err != nil {
		return nil, err
	} else {
		sv.Flags = acroform.SigSeedValueFlags(uint32(ff))
	}

	if filter, err := pdf.Optional(x.GetName(path, dict["Filter"])); err != nil {
		return nil, err
	} else {
		sv.Filter = filter
	}
	if sv.SubFilter, err = readNameArray(x, path, dict["SubFilter"]); err != nil {
		return nil, err
	}
	if sv.DigestMethod, err = readNameArray(x, path, dict["DigestMethod"]); err != nil {
		return nil, err
	}

	if v, err := pdf.Optional(x.GetInteger(path, dict["V"])); err != nil {
		return nil, err
	} else if v >= 1 && v <= 3 {
		sv.V = int(v)
	}

	if cert, err := pdf.ExtractorGetOptional(x, path, dict["Cert"], sigCertSeedValue); err != nil {
		return nil, err
	} else {
		sv.Cert = cert
	}

	if sv.Reasons, err = readTextStringArray(x, path, dict["Reasons"]); err != nil {
		return nil, err
	}

	if mdp, err := pdf.Optional(x.GetDict(path, dict["MDP"])); err != nil {
		return nil, err
	} else if mdp != nil && mdp["P"] != nil {
		// an MDP dict without /P defines no rules, so /P must stay unset
		if p, err := pdf.Optional(x.GetInteger(path, mdp["P"])); err != nil {
			return nil, err
		} else if p >= 0 && p <= 3 {
			sv.MDP.Set(uint(p))
		}
	}

	if ts, err := pdf.Optional(x.GetDict(path, dict["TimeStamp"])); err != nil {
		return nil, err
	} else if ts != nil {
		stamp := &acroform.SigSeedValueTimeStamp{}
		if url, err := pdf.Optional(x.GetString(path, ts["URL"])); err != nil {
			return nil, err
		} else {
			stamp.URL = string(url)
		}
		if ff, err := pdf.Optional(x.GetInteger(path, ts["Ff"])); err != nil {
			return nil, err
		} else {
			stamp.Required = ff == 1
		}
		sv.TimeStamp = stamp
	}

	if sv.LegalAttestation, err = readTextStringArray(x, path, dict["LegalAttestation"]); err != nil {
		return nil, err
	}

	if rev, err := pdf.Optional(x.GetBoolean(path, dict["AddRevInfo"])); err != nil {
		return nil, err
	} else {
		sv.AddRevInfo = bool(rev)
	}

	if ld, err := pdf.Optional(x.GetName(path, dict["LockDocument"])); err != nil {
		return nil, err
	} else if ld == "true" || ld == "false" || ld == "auto" {
		sv.LockDocument = ld
	}

	if af, err := pdf.Optional(pdf.GetTextString(x.R, dict["AppearanceFilter"])); err != nil {
		return nil, err
	} else {
		sv.AppearanceFilter = string(af)
	}

	return sv, nil
}

// sigCertSeedValue reads a certificate seed value dictionary from a PDF file.
func sigCertSeedValue(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*acroform.SigCertSeedValue, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing certificate seed value dictionary")
	}

	cert := &acroform.SigCertSeedValue{SingleUse: isDirect}

	if ff, err := pdf.Optional(x.GetInteger(path, dict["Ff"])); err != nil {
		return nil, err
	} else {
		cert.Flags = acroform.SigCertSeedValueFlags(uint32(ff))
	}

	if cert.Subject, err = readByteStringArray(x, path, dict["Subject"]); err != nil {
		return nil, err
	}
	if cert.Issuer, err = readByteStringArray(x, path, dict["Issuer"]); err != nil {
		return nil, err
	}
	if cert.OID, err = readByteStringArray(x, path, dict["OID"]); err != nil {
		return nil, err
	}
	if cert.SubjectDN, err = readSubjectDN(x, path, dict["SubjectDN"]); err != nil {
		return nil, err
	}
	if cert.KeyUsage, err = readASCIIStringArray(x, path, dict["KeyUsage"]); err != nil {
		return nil, err
	}

	if url, err := pdf.Optional(x.GetString(path, dict["URL"])); err != nil {
		return nil, err
	} else {
		cert.URL = string(url)
	}
	if urlType, err := pdf.Optional(x.GetName(path, dict["URLType"])); err != nil {
		return nil, err
	} else {
		cert.URLType = urlType
	}

	if oid, err := pdf.Optional(x.GetString(path, dict["SignaturePolicyOID"])); err != nil {
		return nil, err
	} else {
		cert.SignaturePolicyOID = string(oid)
	}
	if hash, err := pdf.Optional(x.GetString(path, dict["SignaturePolicyHashValue"])); err != nil {
		return nil, err
	} else if len(hash) > 0 {
		cert.SignaturePolicyHashValue = []byte(hash)
	}
	if alg, err := pdf.Optional(x.GetName(path, dict["SignaturePolicyHashAlgorithm"])); err != nil {
		return nil, err
	} else {
		cert.SignaturePolicyHashAlgorithm = alg
	}
	if cert.SignaturePolicyCommitmentType, err = readASCIIStringArray(x, path, dict["SignaturePolicyCommitmentType"]); err != nil {
		return nil, err
	}

	return cert, nil
}

// readSubjectDN reads the /SubjectDN array of attribute dictionaries.
func readSubjectDN(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]map[pdf.Name]string, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([]map[pdf.Name]string, 0, len(arr))
	for _, el := range arr {
		d, err := pdf.Optional(x.GetDict(path, el))
		if err != nil {
			return nil, err
		}
		if len(d) == 0 {
			continue
		}
		attrs := make(map[pdf.Name]string, len(d))
		for k, v := range d {
			if s, err := pdf.Optional(pdf.GetTextString(x.R, v)); err != nil {
				return nil, err
			} else {
				attrs[k] = string(s)
			}
		}
		out = append(out, attrs)
	}
	return out, nil
}

// shared array read helpers for the signature lock and seed value
// dictionaries. each reader skips malformed elements.

func readNameArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]pdf.Name, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([]pdf.Name, 0, len(arr))
	for _, el := range arr {
		if name, err := pdf.Optional(x.GetName(path, el)); err != nil {
			return nil, err
		} else if name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

func readTextStringArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]string, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		if s, err := pdf.Optional(pdf.GetTextString(x.R, el)); err != nil {
			return nil, err
		} else {
			out = append(out, string(s))
		}
	}
	return out, nil
}

func readASCIIStringArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]string, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		if s, err := pdf.Optional(x.GetString(path, el)); err != nil {
			return nil, err
		} else {
			out = append(out, string(s))
		}
	}
	return out, nil
}

func readByteStringArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([][]byte, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(arr) == 0 {
		return nil, err
	}
	out := make([][]byte, 0, len(arr))
	for _, el := range arr {
		if s, err := pdf.Optional(x.GetString(path, el)); err != nil {
			return nil, err
		} else {
			out = append(out, []byte(s))
		}
	}
	return out, nil
}
