package data_test

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/min1324/data/queue"
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
	EnQueue(interface{}) bool
	DeQueue() (interface{}, bool)
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

// MutexSlice an array of queue
// each time have concurrent Add pushID
// but only one can get and add popID
// if popID == (pushID or 0),it means queue empty,
// return -1
type MutexSlice struct {
	count  uintptr // size
	popID  uintptr // current pop id
	pushID uintptr // current push id

	dirty [mod + 1]queue.SLQueue

	pushMu sync.Mutex
	popMu  sync.Mutex
	once   sync.Once
}

func (s *MutexSlice) hash(id uintptr) *queue.SLQueue {
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

func (s *MutexSlice) EnQueue(i interface{}) bool {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	s.onceInit()

	id := atomic.LoadUintptr(&s.pushID)
	ok := s.hash(id).EnQueue(i)
	if ok {
		atomic.AddUintptr(&s.pushID, 1)
		atomic.AddUintptr(&s.count, 1)
	}
	return ok
}

func (s *MutexSlice) DeQueue() (val interface{}, ok bool) {
	id := atomic.LoadUintptr(&s.popID)
	if id == atomic.LoadUintptr(&s.pushID) {
		return nil, false
	}

	s.popMu.Lock()
	defer s.popMu.Unlock()
	s.onceInit()

	id = atomic.LoadUintptr(&s.popID)
	if id == atomic.LoadUintptr(&s.pushID) {
		return nil, false
	}
	e, ok := s.hash(id).DeQueue()
	if ok {
		atomic.AddUintptr(&s.popID, 1)
		atomic.AddUintptr(&s.count, null)
	}
	return e, ok
}

// UnsafeQueue queue without mutex
type UnsafeQueue struct {
	head, tail *node
	len        int
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
	q.len = 0
}

func (q *UnsafeQueue) Init() {
	q.onceInit()

	head := q.head // start element
	tail := q.tail // end element
	if head == tail {
		return
	}
	q.head = q.tail
	q.len = 0
	// free queue [s ->...-> e]
	for head != tail && head != nil {
		el := head
		head = el.next
		el.next = nil
	}
	return
}

func (q *UnsafeQueue) Size() int {
	return q.len
}

func (q *UnsafeQueue) EnQueue(i interface{}) bool {
	q.onceInit()
	slot := newNode(i)
	q.tail.next = slot
	q.tail = slot
	q.len += 1
	return true
}

func (q *UnsafeQueue) DeQueue() (val interface{}, ok bool) {
	q.onceInit()
	if q.head.next == nil {
		return nil, false
	}
	slot := q.head
	q.head = q.head.next
	slot.next = nil
	q.len -= 1
	val = q.head.load()
	return val, true
}
