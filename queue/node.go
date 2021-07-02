// package queue
// linklist queue stateNode

package queue

import (
	"sync/atomic"
	"unsafe"
)

const (
	geted_stat   uint32 = iota // geted_stat 状态时，已经取出，可以写入。这个状态必须0
	setting_stat               // 只能由EnQUeue操作变成seted_stat, DeQueue操作遇到，说明队列已空.
	seted_stat                 // seted_stat 状态时，已经写入,可以取出。
	getting_stat               // 只能由DeQueue操作变成geted_stat，EnQUeue操作遇到，说明队列满了.
)

// node接口
type node interface {
	load() interface{}
	store(interface{})
	free()
}

func newNode() node {
	return &baseNode{}
}

// interface node
type baseNode struct {
	p interface{}
}

func newBaseNode(i interface{}) *baseNode {
	return &baseNode{p: i}
}

func (n *baseNode) load() interface{} {
	return n.p
}

func (n *baseNode) store(i interface{}) {
	n.p = i
}

func (n *baseNode) free() {
	n.p = nil
}

// unsafe.Pointer node
type unNode struct {
	p unsafe.Pointer
}

func newUnNode(i interface{}) *unNode {
	return &unNode{p: unsafe.Pointer(&i)}
}

func (n *unNode) load() interface{} {
	p := atomic.LoadPointer(&n.p)
	if p == nil {
		return nil
	}
	return *(*interface{})(p)
}

func (n *unNode) store(i interface{}) {
	atomic.StorePointer(&n.p, unsafe.Pointer(&i))
}

func (n *unNode) free() {
	atomic.StorePointer(&n.p, nil)
}

// 链表节点
type listNode struct {
	baseNode
	next *listNode
}

func newListNode(i interface{}) *listNode {
	ln := listNode{}
	ln.store(i)
	return &ln
}

func (n *listNode) free() {
	n.baseNode.free()
	n.next = nil
}

// node next->unsafe.Pointer
type ptrNode struct {
	p    interface{}
	next unsafe.Pointer
}

func newPrtNode(i interface{}) *ptrNode {
	return &ptrNode{p: i}
}

func (n *ptrNode) load() interface{} {
	return n.p
}

func (n *ptrNode) store(i interface{}) {
	n.p = i
}

func (n *ptrNode) free() {
	n.p = nil
	n.next = nil
}

// node next->unsafe.Pointer
type unListNode struct {
	p    unsafe.Pointer
	next unsafe.Pointer
}

func newUnListNode(i interface{}) *unListNode {
	return &unListNode{p: unsafe.Pointer(&i)}
}

func (n *unListNode) load() interface{} {
	p := atomic.LoadPointer(&n.p)
	if p == nil {
		return nil
	}
	return *(*interface{})(p)
}

func (n *unListNode) store(i interface{}) {
	atomic.StorePointer(&n.p, unsafe.Pointer(&i))
}

func (n *unListNode) free() {
	atomic.StorePointer(&n.p, nil)
	atomic.StorePointer(&n.next, nil)
}
