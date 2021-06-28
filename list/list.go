package list

import (
	"sync/atomic"
	"unsafe"
)

type Element struct {
	p          unsafe.Pointer
	prev, next unsafe.Pointer
	deleted    uintptr // mark element is deleted or deleting
}

func (e *Element) Value() interface{} {
	return *(*interface{})(atomic.LoadPointer(&e.p))
}

// Next returns the item on the right.
func (e *Element) Next() *Element {
	return (*Element)(atomic.LoadPointer(&e.next))
}

// setValue sets the value of the item.
// The value needs to be wrapped in unsafe.Pointer already.
func (e *Element) setValue(value unsafe.Pointer) {
	atomic.StorePointer(&e.p, value)
}

func (e *Element) casValue(old interface{}, new unsafe.Pointer) bool {
	p := atomic.LoadPointer(&e.p)
	if *(*interface{})(p) == old {
		return false
	}
	return atomic.CompareAndSwapPointer(&e.p, p, new)
}

// The zero value for List is an empty list ready to use.
type List struct {
	root  Element
	count uintptr
}

func (l *List) Init() *List {
	l.root.next = unsafe.Pointer(&l.root)
	return l
}

// NewList returns an initialized list.
func NewList() *List {
	return new(List).Init()
}

func (l *List) Len() int {
	if l == nil {
		return 0
	}
	return int(atomic.LoadUintptr(&l.count))
}

// Head returns the head item of the list.
func (l *List) Head() *Element {
	if l == nil {
		return nil
	}
	return &l.root
}

func (l *List) Insert(e, at *Element) {
	if at == nil {
		return
	}
	l.insertAt(e, at, (*Element)(at.next))
}

// insertAt insert e between prev and next
func (l *List) insertAt(e, prev, next *Element) bool {
	if prev == nil {
		// means insert e next to root.
		// root->next == next

		e.next = unsafe.Pointer(next)

		// insert at root,root->next = e.
		if !atomic.CompareAndSwapPointer(&l.root.next, unsafe.Pointer(next), unsafe.Pointer(e)) {
			// item was modified concurrently
			return false
		}
	} else {
		e.next = unsafe.Pointer(next)
		if !atomic.CompareAndSwapPointer(&prev.next, unsafe.Pointer(next), unsafe.Pointer(e)) {
			// item was modified concurrently
			return false
		}
	}
	atomic.AddUintptr(&l.count, 1)
	return true
}
