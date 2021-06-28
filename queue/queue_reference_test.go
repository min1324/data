package queue_test

import (
	"sync"
	"sync/atomic"
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
	// return &node{p: unsafe.Pointer(&i)}
}

func (n *node) load() interface{} {
	return n.p
	//return *(*interface{})(n.p)
}

func (q *MutexQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *MutexQueue) init() {
	q.head = &node{}
	q.tail = q.head
	q.count = 0
}

func (q *MutexQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	s := q.head // start node
	e := q.tail // end node
	if s == e {
		return
	}
	q.head = q.tail
	q.count = 0
	// free queue [s ->...-> e]
	for s != e {
		node := s
		s = node.next
		node.next = nil
	}
	return
}

func (q *MutexQueue) Size() int {
	return q.count
}

func (q *MutexQueue) Push(i interface{}) {
	q.mu.Lock()
	q.onceInit()
	slot := newNode(i)
	q.tail.next = slot
	q.tail = slot
	q.mu.Unlock()
}

func (q *MutexQueue) Pop() interface{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.head.next == nil {
		return nil
	}
	slot := q.head
	q.head = q.head.next
	slot.next = nil
	return q.head.load()
}

const (
	bit  = 3
	mod  = 1<<bit - 1
	null = ^uintptr(0) // -1
)

// MutexSlice an array of queue
// each time have concurrent Add pushID
// but only one can get and add popID
// if popID == (pushID or 0),it means queue empty,
// return -1
type MutexSlice struct {
	dirty  [mod + 1]MutexQueue
	count  uintptr
	pushID uintptr
	popID  uintptr
	pushMu sync.Mutex
	popMu  sync.Mutex
}

func (s *MutexSlice) hash(id uintptr) *MutexQueue {
	return &s.dirty[id&mod]
}

func (s *MutexSlice) Init() {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()

	s.popMu.Lock()
	defer s.popMu.Unlock()

	s.popID = s.pushID
	for i := 0; i < len(s.dirty); i++ {
		s.dirty[i].Init()
	}
	s.count = 0
}

func (s *MutexSlice) Size() int {
	return int(atomic.LoadUintptr(&s.count))
}

func (s *MutexSlice) Push(i interface{}) {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()

	id := atomic.LoadUintptr(&s.pushID)
	s.hash(id).Push(i)
	atomic.AddUintptr(&s.pushID, 1)
	atomic.AddUintptr(&s.count, 1)
}

func (s *MutexSlice) Pop() interface{} {
	id := atomic.LoadUintptr(&s.popID)
	if id == atomic.LoadUintptr(&s.pushID) {
		return nil
	}

	s.popMu.Lock()
	defer s.popMu.Unlock()

	id = atomic.LoadUintptr(&s.popID)
	if id == atomic.LoadUintptr(&s.pushID) {
		return nil
	}
	e := s.hash(id).Pop()
	atomic.AddUintptr(&s.popID, 1)
	atomic.AddUintptr(&s.count, null)
	return e
}

// UnsafeQueue stack without mutex
type UnsafeQueue struct {
	head, tail *node
	count      int
	once       sync.Once
}

func (q *UnsafeQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *UnsafeQueue) init() {
	q.head = &node{}
	q.tail = q.head
	q.count = 0
}

func (q *UnsafeQueue) Init() {
	q.onceInit()

	s := q.head // start node
	e := q.tail // end node
	if s == e {
		return
	}
	q.head = q.tail
	q.count = 0
	// free queue [s ->...-> e]
	for s != e {
		node := s
		s = node.next
		node.next = nil
	}
	return
}

func (q *UnsafeQueue) Size() int {
	return q.count
}

func (q *UnsafeQueue) Push(i interface{}) {
	q.onceInit()
	slot := newNode(i)
	q.tail.next = slot
	q.tail = slot
}

func (q *UnsafeQueue) Pop() interface{} {
	q.onceInit()
	if q.head.next == nil {
		return nil
	}
	slot := q.head
	q.head = q.head.next
	slot.next = nil
	return q.head.load()
}
