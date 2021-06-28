package queue

import (
	"sync"
)

// ---------------------------		queue with slice	-----------------------------//

// SQueue is an unbounded queue which uses a slice as underlying.
type SQueue struct {
	data []interface{}
	mu   sync.Mutex
	once sync.Once
}

func (q *SQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *SQueue) init() {
	q.data = make([]interface{}, 0, mod+1)
}

func (q *SQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	if len(q.data) == 0 {
		return
	}
	q.init()
	// free queue [s ->...-> e]
}

func (q *SQueue) Size() int {
	return len(q.data)
}

func (q *SQueue) Push(i interface{}) {
	q.mu.Lock()
	q.onceInit()
	q.data = append(q.data, i)
	q.mu.Unlock()
}

func (q *SQueue) Pop() interface{} {
	if q.Size() == 0 {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	if q.Size() == 0 {
		return nil
	}
	slot := q.data[0]
	q.data = q.data[1:]
	return slot
}

// ---------------------------	queue with list		-----------------------------//

// element stack,queue element
type element struct {
	// p    unsafe.Pointer
	p    interface{}
	next *element
}

func newElement(i interface{}) *element {
	return &element{p: i}
	// return &element{p: unsafe.Pointer(&i)}
}

func (n *element) load() interface{} {
	return n.p
	//return *(*interface{})(n.p)
}

// MQueue list queue with one mutex

type MQueue struct {
	head  *element
	tail  *element
	count int
	mu    sync.Mutex
	once  sync.Once
}

func (q *MQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *MQueue) init() {
	q.head = &element{}
	q.tail = q.head
	q.count = 0
}

func (q *MQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	s := q.head // start element
	e := q.tail // end element
	if s == e {
		return
	}
	q.head = q.tail
	q.count = 0
	// free queue [s ->...-> e]
	for s != e {
		element := s
		s = element.next
		element.next = nil
	}
	return
}

func (q *MQueue) Size() int {
	return q.count
}

func (q *MQueue) Push(i interface{}) {
	q.mu.Lock()
	q.onceInit()
	slot := newElement(i)
	q.tail.next = slot
	q.tail = slot
	q.mu.Unlock()
}

func (q *MQueue) Pop() interface{} {
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

// DMQueue is a concurrent unbounded queue which uses two-Lock concurrent queue qlgorithm.
type DMQueue struct {
	head   *element
	tail   *element
	count  int
	popMu  sync.Mutex
	pushMu sync.Mutex
	once   sync.Once
}

func (q *DMQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *DMQueue) init() {
	q.head = &element{}
	q.tail = q.head
	q.count = 0
}

func (q *DMQueue) Init() {
	q.pushMu.Lock()
	defer q.pushMu.Unlock()
	q.popMu.Lock()
	defer q.popMu.Unlock()

	q.onceInit()

	s := q.head // start element
	e := q.tail // end element
	if s == e {
		return
	}
	q.head = q.tail
	q.count = 0
	// free queue [s ->...-> e]
	for s != e {
		element := s
		s = element.next
		element.next = nil
	}
	return
}

func (q *DMQueue) Size() int {
	return q.count
}

func (q *DMQueue) Push(i interface{}) {
	q.pushMu.Lock()
	defer q.pushMu.Unlock()

	q.onceInit()
	slot := newElement(i)
	q.tail.next = slot
	q.tail = slot
}

func (q *DMQueue) Pop() interface{} {
	if q.head == q.tail {
		return nil
	}
	q.popMu.Lock()
	defer q.popMu.Unlock()
	q.onceInit()

	if q.head.next == nil {
		return nil
	}
	slot := q.head
	q.head = q.head.next
	slot.next = nil
	return q.head.load()
}
