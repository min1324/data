package stack

import (
	"sync/atomic"
	"unsafe"
)

type Stack interface {
	Push(i interface{}) bool
	Pop() (val interface{}, ok bool)
}

const (
	DefauleSize = 1 << 10
	negativeOne = ^uint32(0) // -1
)

type stackNil *struct{}

// 用一个包装nil值。
var empty = stackNil(nil)

// New return an empty Stack
func New() Stack {
	return &LLStack{}
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
