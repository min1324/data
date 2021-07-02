package stack

import (
	"sync/atomic"
	"unsafe"
)

type Stack interface {
	Push(i interface{}) bool
	Pop() (val interface{}, ok bool)
}

type DataStack interface {
	Stack
	onceInit()
	Init()
	Size() int
	Full() bool
	Empty() bool
}

const (
	DefauleSize = 1 << 10
	negativeOne = ^uint32(0) // -1
)

type stackNil *struct{}

// 用一个包装nil值。
var empty = stackNil(nil)

// New return an empty lock-free unbounded list Stack
func New() Stack {
	return &LLStack{}
}

// lock-free 数组栈
func NewLAStack() Stack {
	return &LAStack{}
}

// lock-free 链表栈
func NewLLStack() Stack {
	return &LLStack{}
}

// 单锁数组栈
func NewSAStack() Stack {
	return &SAStack{}
}

// 单锁链表栈
func NewSLStack() Stack {
	return &SLStack{}
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
