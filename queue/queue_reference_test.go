package queue_test

import (
	"sync"
)

type Interface interface {
	Push(interface{})
	Pop() interface{}
	Init()
}

// MutexQueue stack with mutex
type MutexQueue struct {
	head, tail *node
	count      int
	mu         sync.Mutex
	once       sync.Once
}

type node struct {
	p    interface{}
	next *node
}

func newNode(i interface{}) *node {
	return &node{p: i}
}
func (q *MutexQueue) onceInit() {
	q.once.Do(func() {
		q.Init()
	})
}

func (q *MutexQueue) Init() {
	q.head = &node{}
	q.tail = q.head
	q.count = 0
}

func (q *MutexQueue) Push(i interface{}) {
	q.mu.Lock()
	q.onceInit()

	n := newNode(i)
	n.next = q.tail.next
	q.tail = n
	q.mu.Unlock()
}

func (q *MutexQueue) Pop() interface{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.head.next == nil {
		return nil
	}
	slot := q.head.next
	q.head = slot.next
	return slot.p
}

// UnsafeQueue stack with mutex
type UnsafeQueue struct {
	head, tail *node
	count      int
	once       sync.Once
}

func (q *UnsafeQueue) onceInit() {
	q.once.Do(func() {
		q.Init()
	})
}

func (q *UnsafeQueue) Init() {
	q.head = &node{}
	q.tail = q.head
	q.count = 0
}

func (q *UnsafeQueue) Push(i interface{}) {
	q.onceInit()

	n := newNode(i)
	n.next = q.tail.next
	q.tail = n
}

func (q *UnsafeQueue) Pop() interface{} {
	q.onceInit()
	if q.head.next == nil {
		return nil
	}
	slot := q.head.next
	q.head = slot.next
	return slot.p
}
