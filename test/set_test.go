package data_test

import (
	"fmt"

	"github.com/min1324/data/set"
)

func ExampleInit() {
	var s set.IntSet
	fmt.Println(s.String())
	fmt.Println(s.Has(4))
	fmt.Println(s.Has(0))
	s.Add(0)
	s.Add(1)
	s.Add(3)
	s.Add(4)
	s.Add(63)
	s.Add(64)
	fmt.Println(s.String())
	fmt.Println(s.Len())
	fmt.Println(s.Has(4))
	fmt.Println(s.Has(64))
	fmt.Println(s.Copy().String())
	s.Clear()
	fmt.Println(s.String())
}

func initSet() (p, q *set.IntSet) {
	var s, t set.IntSet
	s.Add(1)
	s.Add(2)
	s.Add(3)

	t.Add(3)
	t.Add(4)
	t.Add(5)
	return &s, &t
}

func ExampleSetUnion() {
	s, t := initSet()
	s.UnionWith(t)
	fmt.Println(s.String())
}

func ExampleSetIntersect() {
	s, t := initSet()
	s.IntersectWith(t)
	fmt.Println(s.String())
}

func ExampleSetDifference() {
	s, t := initSet()
	s.DifferenceWith(t)
	fmt.Println(s.String())
}

func ExampleSetComplement() {
	s, t := initSet()
	s.ComplementWith(t)
	fmt.Println(s.String())
}
