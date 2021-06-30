package queue

import (
	"sync/atomic"
	"unsafe"
)

// Queue interface of queue
type Queue interface {
	// queue iniliazie func,init first use
	Init()

	// queue len
	Size() int

	// put a value into tail of queue
	Push(interface{})

	// get queue head value
	Pop() interface{}
}

const (
	negativeOne = ^uint32(0) // -1
)

var empty = unsafe.Pointer(new(interface{}))

// New return an empty lock-free unbound list Queue
func New() Queue {
	var q LFQueue
	q.onceInit()
	return &q
}

/*
第一个字母:
single mutex	=>	S
dobule mutex	=>	D
lock-free		=>	L

第二个字母:
list	=>	L
array	=>	A
ring array	=>	R
*/

func cas(p *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return atomic.CompareAndSwapPointer(p, old, new)
}

func casUint32(p *uint32, old, new uint32) bool {
	return atomic.CompareAndSwapUint32(p, old, new)
}
