package charcode

import (
	"slices"

	"golang.org/x/exp/maps"
)

// A Decoder efficiently decodes character codes from a byte stream.
type Decoder struct {
	tree []node
}

// node represents a node in the lookup tree of the Decoder.
type node struct {
	// The current input byte is compared to high.  If the input byte is less
	// than or equal to high, the byte is consumed and child node becomes the
	// new node.  Otherwise, the next node in Decoder.tree becomes the new node.
	high byte

	// Child determines the next node if the input byte is less than or equal
	// to high:
	// - If child equals 0, the lookup stops, because the consumed byte
	//   is the last byte of a valid character code.
	// - If child equals MaxUint16-k, for k∈{0,1,2,3}, the lookup stopes
	//   because an invalid character code was encountered.
	//   An additional k bytes must be consumed to continue.
	// - Otherwise, the child node is the index of the next node in Decoder.tree.
	child uint16
}

// NewDecoder returns a new Decoder for the given code space range.
func NewDecoder(csr CodeSpaceRange) *Decoder {
	for _, r := range csr {
		if len(r.Low) != len(r.High) && len(r.Low) > 0 && len(r.Low) <= 4 {
			panic("charcode: invalid code space range")
		}
	}

	ii := make([]int, 0, len(csr))
	for i := range csr {
		ii = append(ii, i)
	}

	tree, _ := newTmpTree(csr, ii, 0)
	b := newBuilder()
	b.Build(tree)

	d := &Decoder{
		tree: b.tree,
	}
	return d
}

// Decode decodes the first character code of an input byte sequence.
// The method returns the character code, the number of bytes consumed,
// and whether the character code is valid.
//
// The return value consumed is always less than or equal to the length of s.
// If the length of s is non-zero, consumed is greater than zero.
//
// At the end of input, (0, 0, false) is returned.
func (d *Decoder) Decode(s []byte) (code uint32, consumed int, valid bool) {
	cur := uint16(0)
	for len(s) > 0 {
		var c byte
		c, s = s[0], s[1:]
		code |= uint32(c) << (8 * consumed)
		consumed++

		for {
			if c <= d.tree[cur].high {
				cur = d.tree[cur].child
				switch cur {
				case leafValid:
					valid = true
					return
				case invalidConsume3:
					consumed += 3
					return 0, consumed, false
				case invalidConsume2:
					consumed += 2
					return 0, consumed, false
				case invalidConsume1:
					consumed++
					return 0, consumed, false
				case invalidConsume0:
					return 0, consumed, false
				}
				break
			}
			cur++
		}
	}

	return 0, consumed, false
}

type tmpTree map[byte]*tmpNode

type tmpNode struct {
	// A string representation of the effect of the sub-tree, used for
	// deduplication.
	//
	// For leaf nodes with valid==false, this is []byte{0x00, consume}.
	//
	// For all other nodes this is []byte{0x01, high1, child1, high2, child2, ..., 0x02},
	//
	// The value is represented as a string so that it can be used as a key in
	// a map.
	desc string

	children tmpTree

	// For leaf nodes, whether a valid code has been found.
	valid bool

	// For leaf nodes with valid==false, this is the number of addional bytes
	// which must be consumed.
	consume int
}

func newTmpTree(csr CodeSpaceRange, ii []int, pos int) (tmpTree, string) {
	t := make(tmpTree)

	minLength := minLength(csr, ii)

	breaks := make(map[int]bool)
	breaks[0] = true
	breaks[256] = true
	for _, i := range ii {
		r := csr[i]
		breaks[int(r.Low[pos])] = true
		breaks[int(r.High[pos])+1] = true
	}
	bSlice := maps.Keys(breaks)
	slices.Sort(bSlice)

	desc := []byte{0x01}

	var kk []int
	for j := 0; j < len(bSlice)-1; j++ {
		low := byte(bSlice[j])
		high := byte(bSlice[j+1] - 1)

		// find all elements of ii which overlap [low, high]
		kk = kk[:0]
		for _, i := range ii {
			r := csr[i]
			if r.Low[pos] <= high && r.High[pos] >= low {
				kk = append(kk, i)
			}
		}

		// At this point we have consumed the input bytes up to pos,
		// we know that any of the ranges in kk could still apply,
		// and bytes after pos must be used to decide which range to use.

		if len(kk) == 0 {
			// There are no ranges in ii which overlap [low, high],
			// so bytes in [low, high] must be invalid.
			t[high] = &tmpNode{
				desc:    string([]byte{0x00, byte(minLength - pos - 1)}),
				valid:   false,
				consume: minLength - pos - 1,
			}
			continue
		}

		isLeaf := true
		for _, k := range kk {
			if len(csr[k].Low) > pos+1 {
				isLeaf = false
				break
			}
		}
		if isLeaf {
			t[high] = &tmpNode{
				desc:  string([]byte{0x01, 0x02}),
				valid: true,
			}
			desc = append(desc, high, 0x01, 0x02)
			continue
		}

		cc, dd := newTmpTree(csr, kk, pos+1)
		t[high] = &tmpNode{
			desc:     dd,
			children: cc,
		}
		desc = append(desc, high)
		desc = append(desc, dd...)
	}

	desc = append(desc, 0x02)
	return t, string(desc)
}

func minLength(csr CodeSpaceRange, ii []int) int {
	if len(ii) == 0 {
		return 1
	}

	min := len(csr[ii[0]].Low)
	for _, i := range ii[1:] {
		if l := len(csr[i].Low); l < min {
			min = l
		}
	}
	return min
}

type builder struct {
	tree []node

	done map[string]uint16
}

func newBuilder() *builder {
	done := make(map[string]uint16)
	done[string([]byte{0x01, 0x02})] = leafValid
	done[string([]byte{0x00, 0x03})] = invalidConsume3
	done[string([]byte{0x00, 0x02})] = invalidConsume2
	done[string([]byte{0x00, 0x01})] = invalidConsume1
	done[string([]byte{0x00, 0x00})] = invalidConsume0

	return &builder{
		done: done,
	}
}

func (b *builder) Build(t tmpTree) uint16 {
	bb := maps.Keys(t)
	slices.Sort(bb)

	// reserve some space
	base := len(b.tree)
	for _, high := range bb {
		b.tree = append(b.tree, node{high: high})
	}

	for i, high := range bb {
		childNode := t[high]

		idx, ok := b.done[childNode.desc]
		if ok {
			b.tree[base+i].child = idx
			continue
		}

		childPos := b.Build(t[high].children)
		b.tree[base+i].child = childPos
		b.done[childNode.desc] = childPos
	}

	return uint16(base)
}

const (
	leafValid       uint16 = 0x0000
	invalidConsume3 uint16 = 0xfffc
	invalidConsume2 uint16 = 0xfffd
	invalidConsume1 uint16 = 0xfffe
	invalidConsume0 uint16 = 0xffff
)
