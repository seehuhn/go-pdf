package pdf

import "fmt"

func asName(obj Object) (Name, error) {
	name, ok := obj.(Name)
	if !ok {
		return "", fmt.Errorf("wrong type, expected Name but got %T", obj)
	}
	return name, nil
}

func asDict(obj Object) (Dict, error) {
	if obj == nil {
		return Dict{}, nil
	}
	dict, ok := obj.(Dict)
	if !ok {
		return nil, fmt.Errorf("wrong type, expected Dict but got %T", obj)
	}
	return dict, nil
}
