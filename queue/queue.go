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
	DefauleSize = 1 << 10
	negativeOne = ^uint32(0) // -1
)

var empty = unsafe.Pointer(new(interface{}))

// New return an empty lock-free unbound list Queue
func New() Queue {
	var q LLQueue
	q.onceInit()
	return &q
}

func cas(p *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return atomic.CompareAndSwapPointer(p, old, new)
}

func casUint32(p *uint32, old, new uint32) bool {
	return atomic.CompareAndSwapUint32(p, old, new)
}

func casUintptr(addr *uintptr, old, new uintptr) bool {
	return atomic.CompareAndSwapUintptr(addr, old, new)
}

// 溢出环形计算需要，得出2^n-1。(2^n>=u,具体可见kfifo）
func minMod(u uintptr) uintptr {
	u -= 1 //兼容0, as min as ,128->127 !255
	u |= u >> 1
	u |= u >> 2
	u |= u >> 4
	u |= u >> 8  // 32位类型已经足够
	u |= u >> 16 // 64位
	return u
}

// 溢出环形计算需要，得出2^n-1。(2^n>=u,具体可见kfifo）
func modUint32(u uint32) uint32 {
	u -= 1 //兼容0, as min as ,128->127 !255
	u |= u >> 1
	u |= u >> 2
	u |= u >> 4
	u |= u >> 8  // 32位类型已经足够
	u |= u >> 16 // 64位
	return u
}
