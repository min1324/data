package data_test

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// use for slice
const (
	bit  = 3
	mod  = 1<<bit - 1
	null = ^uintptr(0) // -1
)

// SQInterface use in stack,queue testing
type SQInterface interface {
	Init()
	Size() int
	Push(interface{})
	Pop() interface{}
}

// node stack,queue node
type node struct {
	// p    unsafe.Pointer
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

// -------------------------------------	stack	------------------------------------------- //

// MutexStack stack with mutex
type MutexStack struct {
	top   *node
	count int
	mu    sync.Mutex
}

func (s *MutexStack) Size() int {
	return s.count
}

func (s *MutexStack) Init() {
	s.mu.Lock()
	e := s.top
	s.top = nil
	s.count = 0
	for e != nil {
		n := e
		e = n.next
		n.next = nil
	}
	s.mu.Unlock()
	runtime.GC()
}

func (s *MutexStack) Push(i interface{}) {
	s.mu.Lock()
	n := newNode(i)
	n.next = s.top
	s.top = n
	s.count++
	s.mu.Unlock()
}

func (s *MutexStack) Pop() interface{} {
	if s.top == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.top == nil {
		return nil
	}
	top := s.top
	s.top = top.next
	s.count--
	return top.p
}

// -------------------------------------	queue	------------------------------------------- //

// MutexQueue stack with mutex
type MutexQueue struct {
	head, tail *node
	count      int
	mu         sync.Mutex
	once       sync.Once
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
	if q.head == q.tail {
		return nil
	}
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

// MutexSlice an array of queue
// each time have concurrent Add pushID
// but only one can get and add popID
// if popID == (pushID or 0),it means queue empty,
// return -1
type MutexSlice struct {
	count  uintptr // size
	popID  uintptr // current pop id
	pushID uintptr // current push id

	dirty [mod + 1]MutexQueue

	pushMu sync.Mutex
	popMu  sync.Mutex
	once   sync.Once
}

func (s *MutexSlice) hash(id uintptr) *MutexQueue {
	return &s.dirty[id&mod]
}

func (s *MutexSlice) onceInit() {
	s.once.Do(func() {
		s.init()
	})
}

func (s *MutexSlice) init() {
	for i := 0; i < len(s.dirty); i++ {
		s.dirty[i].Init()
	}
	s.popID = s.pushID
	s.count = 0
}

// Init prevent push new element into queue
func (s *MutexSlice) Init() {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()

	s.popMu.Lock()
	defer s.popMu.Unlock()

	s.init()
}

func (s *MutexSlice) Size() int {
	return int(atomic.LoadUintptr(&s.count))
}

func (s *MutexSlice) Push(i interface{}) {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	s.onceInit()

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
	s.onceInit()

	id = atomic.LoadUintptr(&s.popID)
	if id == atomic.LoadUintptr(&s.pushID) {
		return nil
	}
	e := s.hash(id).Pop()
	atomic.AddUintptr(&s.popID, 1)
	atomic.AddUintptr(&s.count, null)
	return e
}

// UnsafeQueue queue without mutex
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
