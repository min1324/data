package queue

import (
	"sync/atomic"
	"unsafe"
)

// LKQueue is a lock-free unbounded queue.
type LKQueue struct {
	len  uint32
	head unsafe.Pointer
	tail unsafe.Pointer
}
type lkNode struct {
	value interface{}
	next  unsafe.Pointer
}

// NewLKQueue returns an empty queue.
func NewLKQueue() *LKQueue {
	n := unsafe.Pointer(&lkNode{})
	return &LKQueue{head: n, tail: n}
}

// Enqueue puts the given value v at the tail of the queue.
func (q *LKQueue) EnQueue(v interface{}) bool {
	n := &lkNode{value: v}
	for {
		tail := loadlkNode(&q.tail)
		next := loadlkNode(&tail.next)
		if tail == loadlkNode(&q.tail) { // are tail and next consistent?
			if next == nil {
				if caslkNode(&tail.next, next, n) {
					caslkNode(&q.tail, tail, n) // Enqueue is done.  try to swing tail to the inserted lkNode
					atomic.AddUint32(&q.len, 1)
					return true
				}
			} else { // tail was not pointing to the last lkNode
				// try to swing Tail to the next lkNode
				caslkNode(&q.tail, tail, next)
			}
		}
	}
}

// Dequeue removes and returns the value at the head of the queue.
// It returns nil if the queue is empty.
func (q *LKQueue) DeQueue() (val interface{}, ok bool) {
	for {
		head := loadlkNode(&q.head)
		tail := loadlkNode(&q.tail)
		next := loadlkNode(&head.next)
		if head == loadlkNode(&q.head) { // are head, tail, and next consistent?
			if head == tail { // is queue empty or tail falling behind?
				if next == nil { // is queue empty?
					return nil, false
				}
				// tail is falling behind.  try to advance it
				caslkNode(&q.tail, tail, next)
			} else {
				// read value before CAS otherwise another dequeue might free the next lkNode
				v := next.value
				if caslkNode(&q.head, head, next) {
					atomic.AddUint32(&q.len, negativeOne)
					atomic.StorePointer(&head.next, nil)
					return v, true // Dequeue is done.  return
				}
			}
		}
	}
}

func (q *LKQueue) Init() {
	n := unsafe.Pointer(&lkNode{})
	q.tail = n
	q.head = n
	q.len = 0
}

func (q *LKQueue) Size() int {
	return int(q.len)
}

func loadlkNode(p *unsafe.Pointer) (n *lkNode) {
	return (*lkNode)(atomic.LoadPointer(p))
}

func caslkNode(p *unsafe.Pointer, old, new *lkNode) (ok bool) {
	return atomic.CompareAndSwapPointer(
		p, unsafe.Pointer(old), unsafe.Pointer(new))
}
