package stack

import (
	"sync/atomic"
	"unsafe"
)

// Stack a lock-free concurrent FILO stack.
type Stack struct {
	top   unsafe.Pointer // point to the latest value pushed.
	count int32          // stack value num.
}

type node struct {
	value interface{}    // value store
	next  unsafe.Pointer // next node
}

// New return an empty Stack
func New() *Stack {
	n := unsafe.Pointer(&node{})
	return &Stack{top: n}
}

// Init initialize stack.
func (s *Stack) Init() {
	s.top = unsafe.Pointer(&node{})
}

// Size stack element's number
func (s *Stack) Size() int32 {
	return atomic.LoadInt32(&s.count)
}

// Push puts the given value at the top of the stack.
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

// Pop removes and returns the value at the top of the stack.
// It returns nil if the stack is empty.
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
