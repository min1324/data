package stack

import (
	"sync/atomic"
	"unsafe"
)

// LLStack a lock-free concurrent FILO stack.
type LLStack struct {
	len uint32         // stack value num.
	top unsafe.Pointer // point to the latest value pushed.
}

// Init initialize stack.
func (s *LLStack) Init() {
	top := atomic.LoadPointer(&s.top)
	for top != nil {
		oldLen := atomic.LoadUint32(&s.len)
		if cas(&s.top, top, nil) {
			atomic.AddUint32(&s.len, (^oldLen + 1))
			break
		}
		top = atomic.LoadPointer(&s.top)
	}
	// free dummy node
	for top != nil {
		freeNode := (*ptrNode)(top)
		top = freeNode.next
		freeNode.free()
	}
}

// Size stack element's number
func (s *LLStack) Size() int {
	return int(atomic.LoadUint32(&s.len))
}

// Push puts the given value at the top of the stack.
func (s *LLStack) Push(val interface{}) bool {
	if val == nil {
		val = empty
	}
	slot := newPrtNode(val)
	for {
		slot.next = atomic.LoadPointer(&s.top)
		if cas(&s.top, slot.next, unsafe.Pointer(slot)) {
			atomic.AddUint32(&s.len, 1)
			break
		}
	}
	return true
}

// Pop removes and returns the value at the top of the stack.
// It returns nil if the stack is empty.
func (s *LLStack) Pop() (val interface{}, ok bool) {
	var slot *ptrNode
	for {
		top := atomic.LoadPointer(&s.top)
		if top == nil {
			return
		}
		slot = (*ptrNode)(top)
		if cas(&s.top, top, slot.next) {
			atomic.AddUint32(&s.len, ^uint32(0))
			break
		}
	}
	val = slot.load()
	if val == empty {
		val = nil
	}
	slot.free()
	return val, true
}
