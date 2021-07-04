package queue

import (
	"sync/atomic"
	"unsafe"
)

// Queue interface of queue
type Queue interface {
	EnQueue(interface{}) bool
	DeQueue() (val interface{}, ok bool)
}

type DataQueue interface {
	Queue
	onceInit()
	Init()
	Cap() int
	Size() int
	Full() bool
	Empty() bool
}

const (
	DefauleSize = 1 << 10
	negativeOne = ^uint32(0) // -1
)

// queueNil is used in queue to represent interface{}(nil).
// Since we use nil to represent empty slots, we need a sentinel value
// to represent nil.
//
// 包装空值
// type queueNil *struct{}

// 用一个包装nil值。
var empty = unsafe.Pointer(new(interface{}))

// New return an empty lock-free unbound list Queue
func New() Queue {
	var q LLQueue
	q.onceInit()
	return &q
}

// 双锁链表队列
func NewDLQueue() Queue {
	var q LLQueue
	q.onceInit()
	return &q
}

// 双锁环形队列
func NewDRQueue() Queue {
	var q DRQueue
	q.onceInit()
	return &q
}

// lock-free 链表队列
func NewLLQueue() Queue {
	var q LLQueue
	q.onceInit()
	return &q
}

func NewLRQueue() Queue {
	var q LRQueue
	q.onceInit()
	return &q
}

// 单锁数组队列
func NewSAQueue() Queue {
	var q SAQueue
	q.onceInit()
	return &q
}

// 单锁链表队列
func NewSLQueue() Queue {
	var q SLQueue
	q.onceInit()
	return &q
}

// 单锁环形队列
func NewSRQueue() Queue {
	var q SRQueue
	q.onceInit()
	return &q
}

// 一组队列
func NewSlice() DataQueue {
	var q Slice
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
