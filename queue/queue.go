package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// Queue a lock-free concurrent IFO queues.
type Queue struct {
	head  unsafe.Pointer // head not store value,the frist value in head.next if exists.
	tail  unsafe.Pointer // tail.next if not nil, need promote.
	count int32          // count is num of value store in queue
	mux   sync.Mutex     // use for init
}

type node struct {
	value interface{}
	next  unsafe.Pointer
}

// New return an empty Queue
func New() *Queue {
	n := unsafe.Pointer(&node{})
	return &Queue{head: n, tail: n}
}

// Init initialize queue in lock.
func (q *Queue) Init() {
	q.mux.Lock()
	q.head = unsafe.Pointer(&node{})
	q.tail = q.head
	q.count = 0
	defer q.mux.Unlock()
}

// Size queue element's number
func (q *Queue) Size() int {
	return int(q.count)
}

// Enqueue puts the given value at the tail of the queue.
func (q *Queue) EnQueue(v interface{}) {
	n := &node{value: v, next: nil}
	for {
		tail := load(&q.tail)
		next := load(&tail.next)

		if tail == load(&q.tail) { // are tail and next consistent?
			if next == nil { // Was Tail pointing to the last node?
				if cas(&tail.next, next, n) { // to link node at the end of the linked list
					atomic.AddInt32(&q.count, 1)
					cas(&q.tail, tail, n) // try to swing tail to the inserted node
					break
				}
			} else {
				cas(&q.tail, tail, next) // try to swing tail to the inserted node
			}
		}
	}
}

// Dequeue removes and returns the value at the head of the queue.
// It returns nil if the queue is empty.
func (q *Queue) DeQueue() interface{} {
	for {
		tail := load(&q.tail)
		head := load(&q.head)
		next := load(&head.next)

		if head == load(&q.head) { // are head, tail, and next consistent?
			if head == tail { // Is queue empty or Tail falling behind?
				if next == nil { // Is queue empty?
					return nil // Queue is empty, couldn't dequeue
				}
				cas(&q.tail, tail, next) // Tail is falling behind.Try to advance it
			} else {
				v := next.value               // Read value before CAS,Otherwise, another dequeue might free the next node
				if cas(&q.head, head, next) { // Try to swing Head to the next node
					atomic.AddInt32(&q.count, -1)
					return v // Dequeue is done.  Exit loop
				}
			}
		}
	}
}

func load(i *unsafe.Pointer) *node {
	return (*node)(atomic.LoadPointer(i))
}

func cas(p *unsafe.Pointer, old, new *node) bool {
	return atomic.CompareAndSwapPointer(
		p, unsafe.Pointer(old), unsafe.Pointer(new))
}
