// Package group implements PDF group XObject attributes.
//
// A group XObject is a form XObject with a Group entry in its dictionary.
// The Group entry contains a group attributes dictionary that specifies
// the properties of the group.
//
// Currently, the only defined group subtype is Transparency, represented
// by [TransparencyAttributes]. Transparency groups control how objects
// are composited together before being blended with the backdrop.
//
// Transparency groups can be associated with:
//   - Pages (via the Group entry in the page dictionary)
//   - Form XObjects (via the Group entry in the form dictionary)
package group
