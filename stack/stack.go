package stack

import (
	"runtime"
	"sync/atomic"
	"unsafe"
)

// Stack a lock-free concurrent FILO stack.
type Stack struct {
	top   unsafe.Pointer // point to the latest value pushed.
	count uintptr        // stack value num.
}

// New return an empty Stack
func New() *Stack {
	return &Stack{}
}

type node struct {
	p    interface{}    // value store
	next unsafe.Pointer // next node
}

func newNode(i interface{}) *node {
	return &node{p: unsafe.Pointer(&i)}
}

// Init initialize stack.
func (s *Stack) Init() {
	e := atomic.LoadPointer(&s.top)
	for e != nil {
		e = atomic.LoadPointer(&s.top)
		if cas(&s.top, e, nil) {
			// free dummy node
			for e != nil {
				node := (*node)(e)
				e = node.next
				node.next = nil
			}
			runtime.GC()
			return
		}
	}
}

// Size stack element's number
func (s *Stack) Size() int {
	return int(atomic.LoadUintptr(&s.count))
}

// Push puts the given value at the top of the stack.
func (s *Stack) Push(i interface{}) {
	slot := newNode(i)
	for {
		slot.next = atomic.LoadPointer(&s.top)
		if cas(&s.top, slot.next, unsafe.Pointer(slot)) {
			atomic.AddUintptr(&s.count, 1)
			break
		}
	}
}

// Pop removes and returns the value at the top of the stack.
// It returns nil if the stack is empty.
func (s *Stack) Pop() interface{} {
	for {
		top := atomic.LoadPointer(&s.top)
		if top == nil {
			break
		}
		slot := (*node)(top)
		if cas(&s.top, top, slot.next) {
			slot.next = nil
			atomic.AddUintptr(&s.count, ^uintptr(0))
			return slot.load()
		}
	}
	return nil
}

func (n *node) load() interface{} {
	return n.p
	// return *(*interface{})(n.p)
}

func cas(addr *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return atomic.CompareAndSwapPointer(addr, old, new)
}
