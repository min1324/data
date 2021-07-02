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

type baseNode struct {
	// TODO 用unsafe.Pointer？
	// p interface{}
	p unsafe.Pointer
}

func newBaseNode(i interface{}) *baseNode {
	// return &baseNode{p: i}
	return &baseNode{p: unsafe.Pointer(&i)}
}

func (n *baseNode) load() interface{} {
	// return n.p
	p := atomic.LoadPointer(&n.p)
	if p == nil {
		return nil
	}
	return *(*interface{})(p)
}

func (n *baseNode) store(i interface{}) {
	// n.p = i
	atomic.StorePointer(&n.p, unsafe.Pointer(&i))
}

// 释放stateNode
func (n *baseNode) free() {
	n.p = nil
}

// 链表节点
type listNode struct {
	// TODO 用unsafe.Pointer？
	baseNode
	next *listNode
}

func newListNode(i interface{}) *listNode {
	ln := listNode{}
	ln.store(i)
	return &ln
	// return &stateNode{p: unsafe.Pointer(&i)}
}

// 释放listNode
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
type unsafeNode struct {
	p    unsafe.Pointer
	next unsafe.Pointer
}

func newUnsafeNode(i interface{}) *unsafeNode {
	return &unsafeNode{p: unsafe.Pointer(&i)}
}

func (n *unsafeNode) load() interface{} {
	p := atomic.LoadPointer(&n.p)
	if p == nil {
		return nil
	}
	return *(*interface{})(p)
}

func (n *unsafeNode) store(i interface{}) {
	atomic.StorePointer(&n.p, unsafe.Pointer(&i))
}

func (n *unsafeNode) free() {
	atomic.StorePointer(&n.p, nil)
	atomic.StorePointer(&n.next, nil)
}

// 包含状态的node
type stateNode struct {
	baseNode

	// state标志node是否可用
	// geted_stat 状态时，已经取出，可以写入。
	// EnQUeue操作通过cas改变为EnQUeueing_stat,获得写入权限。
	// 如无需权限，直接变为EnQUeueed.
	//
	// seted_stat 状态时，已经写入,可以取出。
	// DeQueue操作通过cas改变为DeQueueing_stat,获得取出权限。
	// 如无需权限，直接变为DeQueueed.
	state uint32
	next  unsafe.Pointer
}

func newStateNode(i interface{}) *stateNode {
	sn := stateNode{}
	sn.baseNode.store(i)
	return &sn
	// return &node{p: unsafe.Pointer(&i)}
}

// 释放node,状态变为:geted_stat
func (n *stateNode) free() {
	n.baseNode.free()
	n.state = geted_stat
	n.next = nil
}

// 更变node的状态
func (n *stateNode) change(s uint32) {
	atomic.StoreUint32(&n.state, s)
}

// 获取node的状态
func (n *stateNode) status() uint32 {
	return atomic.LoadUint32(&n.state)
}
