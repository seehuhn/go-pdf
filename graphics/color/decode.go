package color

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

func DecodeSpace(r pdf.Getter, desc pdf.Object) (Space, error) {
	desc, err := pdf.Resolve(r, desc)
	if err != nil {
		return nil, err
	}

	switch desc := desc.(type) {
	case pdf.Name:
		switch desc {
		case FamilyDeviceGray:
			return DeviceGray, nil
		case FamilyDeviceRGB:
			return DeviceRGB, nil
		case FamilyDeviceCMYK:
			return DeviceCMYK, nil
		case FamilyPattern:
			return spacePatternColored{}, nil
		}
	case pdf.Array:
		if len(desc) == 0 {
			break
		}
		name, err := pdf.GetName(r, desc[0])
		if pdf.IsMalformed(err) {
			break
		} else if err != nil {
			return nil, err
		}

		switch name {
		case FamilyCalGray:
			if len(desc) != 2 {
				break
			}
			dict, err := pdf.GetDict(r, desc[1])
			if err != nil {
				break
			}

			whitePoint, err := getArrayN(r, dict["WhitePoint"], 3)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}

			blackPoint, err := getArrayN(r, dict["BlackPoint"], 3)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if blackPoint == nil {
				blackPoint = []float64{0, 0, 0}
			}

			gamma, err := pdf.GetNumber(r, dict["Gamma"])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}

			res := &SpaceCalGray{
				whitePoint: whitePoint,
				blackPoint: blackPoint,
				gamma:      float64(gamma),
			}
			return res, nil
		case "CalCMYK": // deprecated
			return DeviceCMYK, nil
		}
	}
	return nil, &pdf.MalformedFileError{
		Err: fmt.Errorf("invalid color space: %s", pdf.Format(desc)),
	}
}

func getArrayN(r pdf.Getter, obj pdf.Object, n int) ([]float64, error) {
	arr, err := pdf.GetArray(r, obj)
	if err != nil {
		return nil, err
	}

	if len(arr) != n {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected array of length %d, got %d", n, len(arr)),
		}
	}

	res := make([]float64, n)
	for i, elem := range arr {
		x, err := pdf.GetNumber(r, elem)
		if err != nil {
			return nil, err
		}
		res[i] = float64(x)
	}
	return res, nil
}
