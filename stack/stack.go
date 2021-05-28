package stack

import (
	"sync/atomic"
	"unsafe"
)

type Stack struct {
	top   unsafe.Pointer
	count int32
}

type node struct {
	value interface{}
	next  unsafe.Pointer
}

func New() *Stack {
	n := unsafe.Pointer(&node{})
	return &Stack{top: n}
}

func (s *Stack) Init() {
	s.top = unsafe.Pointer(&node{})
}

func (s *Stack) Size() int32 {
	return atomic.LoadInt32(&s.count)
}

func (s *Stack) Push(i interface{}) {
	slot := &node{}
	for {
		// give a slot frist,then store value
		slot.next = atomic.LoadPointer(&s.top)
		if cas(&s.top, slot.next, unsafe.Pointer(slot)) {
			slot.value = i
			atomic.AddInt32(&s.count, 1)
			break
		}
	}
}

func (s *Stack) Pop() interface{} {
	for {
		slot := load(&s.top)
		if slot.next == nil {
			break
		}
		if cas(&s.top, unsafe.Pointer(slot), slot.next) {
			slot.next = nil
			atomic.AddInt32(&s.count, -1)
			return slot.value
		}
	}
	return nil
}

func load(i *unsafe.Pointer) *node {
	return (*node)(atomic.LoadPointer(i))
}

func cas(addr *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return atomic.CompareAndSwapPointer(addr, old, new)
}
