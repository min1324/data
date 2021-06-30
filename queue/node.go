// package queue
// linklist queue stateNode

package queue

import (
	"sync/atomic"
	"unsafe"
)

const (
	poped_stat   uint32 = iota // poped_stat 状态时，已经取出，可以写入。这个状态必须0
	pushing_stat               // 只能由push操作变成pushed_stat, pop操作遇到，说明队列已空.
	pushed_stat                // pushed_stat 状态时，已经写入,可以取出。
	poping_stat                // 只能由pop操作变成poped_stat，push操作遇到，说明队列满了.
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
	p interface{}
}

func newBaseNode(i interface{}) *baseNode {
	return &baseNode{p: i}
	// return &stateNode{p: unsafe.Pointer(&i)}
}

func (n *baseNode) load() interface{} {
	return n.p
	//return *(*interface{})(n.p)
}

func (n *baseNode) store(i interface{}) {
	n.p = i
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
	baseNode
	next unsafe.Pointer
}

func newPrtNode(i interface{}) *ptrNode {
	pn := ptrNode{}
	pn.store(i)
	return &pn
	// return &stateNode{p: unsafe.Pointer(&i)}
}

// 释放ptrNode
func (n *ptrNode) free() {
	n.baseNode.free()
	n.next = nil
}

// 包含状态的node
type stateNode struct {
	baseNode

	// state标志node是否可用
	// poped_stat 状态时，已经取出，可以写入。
	// push操作通过cas改变为pushing_stat,获得写入权限。
	// 如无需权限，直接变为pushed.
	//
	// pushed_stat 状态时，已经写入,可以取出。
	// pop操作通过cas改变为poping_stat,获得取出权限。
	// 如无需权限，直接变为poped.
	state uint32
	next  unsafe.Pointer
}

func newStateNode(i interface{}) *stateNode {
	sn := stateNode{}
	sn.baseNode.store(i)
	return &sn
	// return &node{p: unsafe.Pointer(&i)}
}

// 释放node
func (n *stateNode) free() {
	n.baseNode.free()
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
