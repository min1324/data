// package queue
// linklist queue node

package queue

import (
	"sync/atomic"
	"unsafe"
)

const (
	node_poped_stat  uint32 = iota // poped_stat 状态时，已经取出，可以写入。
	node_pushed_stat               // pushed_stat 状态时，已经写入,可以取出。
)

type node struct {
	// stat标志node是否可用
	// poped_stat 状态时，已经取出，可以写入。push操作通过cas改变为pushing_stat,获得写入权限。
	// pushed_stat 状态时，已经写入,可以取出。pop操作通过cas改变为poping_stat,获得取出权限。
	stat uint32
	next unsafe.Pointer

	// TODO 用unsafe.Pointer？
	p interface{}
}

func newNode(i interface{}) *node {
	return &node{p: i}
	// return &node{p: unsafe.Pointer(&i)}
}

func (n *node) load() interface{} {
	return n.p
	//return *(*interface{})(n.p)
}

// 释放node
func (n *node) free() {
	n.next = nil
	n.p = nil
}

// 更变node的状态
func (n *node) change(s uint32) {
	atomic.StoreUint32(&n.stat, s)
}

// 获取node的状态
func (n *node) status() uint32 {
	return atomic.LoadUint32(&n.stat)
}
