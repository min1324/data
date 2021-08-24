package set

import (
	"bytes"
	"fmt"
)

var (
	// platform bit = 2^setBits,(32/64)
	setBits  = 5 + (^uint(0) >> 63)
	platform = 1 << setBits
	setMesk  = 1<<setBits - 1
)

// IntSet is a set of non-negative integers.
// Its zero value represents the empty set.
//
// x is an item in set.
// x = (2^setBits)*idx + mod <==> x = 64*idx + mod  or  x = 32*idx + mod
// idx = x/2^setBits (x>>setBits) , mod = x%2^setBits (x&setMesk)
// in the set, x is the pesition: dirty[idx]&(1<<mod)
type IntSet struct {
	dirty []uint
}

// Len return the number of elements in set
func (s *IntSet) Len() int {
	var sum int
	for _, e := range s.dirty {
		for ; e > 0; e >>= 1 {
			if e&1 == 1 {
				sum += 1
			}
		}
	}
	return sum
}

// Clear remove all elements from the set
func (s *IntSet) Clear() {
	s.dirty = make([]uint, 0, 1<<(10-setBits))
}

// Copy return a copy of the set
func (s *IntSet) Copy() *IntSet {
	var n IntSet
	n.dirty = make([]uint, len(s.dirty))
	for i := range s.dirty {
		n.dirty[i] = s.dirty[i]
	}
	return &n
}

// Items return all element in the set
func (s *IntSet) Items() []int {
	sum := 0
	array := make([]int, len(s.dirty)*platform)

	for i, item := range s.dirty {
		if item == 0 {
			continue
		}
		for j := 0; j < platform; j++ {
			if item&(1<<uint(j)) != 0 {
				array[sum] = 1<<(platform+i) + j
				sum += 1
			}
		}
	}
	return array[:sum]
}

// Null report s if an empty set
func (s *IntSet) Null() bool {
	for i := range s.dirty {
		if s.dirty[i] != 0 {
			return false
		}
	}
	return true
}

// Equal return if s <==> t
func (s *IntSet) Equal(t *IntSet) bool {
	if len(s.dirty) != len(t.dirty) {
		return false
	}
	for i := range s.dirty {
		if s.dirty[i] != t.dirty[i] {
			return false
		}
	}
	return true
}

// String returns the set as a string of the form "{1 2 3}".
func (s *IntSet) String() string {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, item := range s.dirty {
		if item == 0 {
			continue
		}
		for j := 0; j < platform; j++ {
			if item&(1<<uint(j)) != 0 {
				if buf.Len() > len("{") {
					buf.WriteByte(' ')
				}
				fmt.Fprintf(&buf, "%d", 1<<(platform+i)+j)
			}
		}
	}
	buf.WriteByte('}')
	return buf.String()
}

// in 64 bit platform
// x = 64*idx + mod
// idx = x/64 (x>>6) , mod = x%64 (x&(1<<6-1))
//
// in 32 bit platform
// x = 32*idx + mod
// idx = x/32 (x>>5) , mod = x%32 (x&(1<<5-1))
func idxMod(x int) (idx, mod int) {
	return x >> setBits, x & setMesk
}

// Has reports whether the set contains the non-negative value x.
func (s *IntSet) Has(x int) bool {
	idx, mod := idxMod(x)
	if idx >= len(s.dirty) {
		// overflow
		return false
	}
	// return s.dirty[idx]&(1<<mod) != 0
	return (s.dirty[idx]>>mod)&1 == 1
}

// Add adds the non-negative value x to the set.
func (s *IntSet) Add(x int) {
	idx, mod := idxMod(x)
	for idx >= len(s.dirty) {
		s.dirty = append(s.dirty, 0)
	}
	s.dirty[idx] |= 1 << mod
}

func (s *IntSet) Adds(args ...int) {
	for _, x := range args {
		s.Add(x)
	}
}

func (s *IntSet) Removes(args ...int) {
	for _, x := range args {
		s.Remove(x)
	}
}

// remove x from the set
func (s *IntSet) Remove(x int) {
	idx, mod := idxMod(x)
	if idx >= len(s.dirty) {
		// overflow
		return
	}
	s.dirty[idx] &^= 1 << mod
}

// Equal return if s <==> t
func Equal(s, t *IntSet) bool {
	if len(s.dirty) != len(t.dirty) {
		return false
	}
	for i := range s.dirty {
		if s.dirty[i] != t.dirty[i] {
			return false
		}
	}
	return true
}

// UnionWith sets s to the union of s and t.
// item in s or t,
func (s *IntSet) UnionWith(t *IntSet) {
	for i, dirty := range t.dirty {
		if i < len(s.dirty) {
			s.dirty[i] |= dirty
		} else {
			s.dirty = append(s.dirty, dirty)
		}
	}
}

// IntersectWith s to the intersection of s and t
// item in s and t,
func (s *IntSet) IntersectWith(t *IntSet) {
	for i, dirty := range t.dirty {
		if i < len(s.dirty) {
			s.dirty[i] &= dirty
		}
	}
}

// DifferenceWith s to the difference of s and t
// item in s and not in t,
func (s *IntSet) DifferenceWith(t *IntSet) {
	for i, dirty := range t.dirty {
		if i < len(s.dirty) {
			s.dirty[i] &^= dirty
		}
	}
}

// ComplementWith s to the complement of s and t
// item in s but not in t, and not in s but in t.
func (s *IntSet) ComplementWith(t *IntSet) {
	for i, dirty := range t.dirty {
		if i < len(s.dirty) {
			s.dirty[i] ^= dirty
		}
	}
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// Union return the union set of s and t.
func Union(s, t *IntSet) *IntSet {
	var p IntSet
	newLen := max(len(s.dirty), len(t.dirty))
	p.dirty = make([]uint, newLen)
	for i := range s.dirty {
		p.dirty[i] = s.dirty[i]
	}

	for i, dirty := range t.dirty {
		p.dirty[i] |= dirty
	}
	return &p
}

// Intersect return the intersection set of s and t
// item in s and t
func Intersect(s, t *IntSet) *IntSet {
	var p IntSet
	newLen := max(len(s.dirty), len(t.dirty))
	p.dirty = make([]uint, newLen)
	for i := range s.dirty {
		p.dirty[i] = s.dirty[i]
	}

	for i, dirty := range t.dirty {
		p.dirty[i] &= dirty
	}
	return &p
}

// Difference return the difference set of s and t
// item in s and not in t
func Difference(s, t *IntSet) *IntSet {
	var p IntSet
	newLen := max(len(s.dirty), len(t.dirty))
	p.dirty = make([]uint, newLen)
	for i := range s.dirty {
		p.dirty[i] = s.dirty[i]
	}

	for i, dirty := range t.dirty {
		p.dirty[i] &^= dirty
	}
	return &p
}

// Complement return the complement set of s and t
// item in s but not in t, and not in s but in t.
func Complement(s, t *IntSet) *IntSet {
	var p IntSet
	newLen := max(len(s.dirty), len(t.dirty))
	p.dirty = make([]uint, newLen)
	for i := range s.dirty {
		p.dirty[i] = s.dirty[i]
	}

	for i, dirty := range t.dirty {
		p.dirty[i] ^= dirty
	}
	return &p
}
