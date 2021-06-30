package queue

import (
	"sync"
	"sync/atomic"
)

// BUG

const (
	bit = 3
	mod = 1<<bit - 1
)

// Slice an array of queue
type Slice struct {
	len    uint32  // size
	popID  uintptr // current pop id
	pushID uintptr // current push id

	dirty [mod + 1]Queue
	New   func() Queue

	pushMu sync.Mutex
	popMu  sync.Mutex
	once   sync.Once
}

func (s *Slice) hash(id uintptr) Queue {
	return s.dirty[id&mod]
}

func (s *Slice) onceInit() {
	s.once.Do(func() {
		s.init()
	})
}

func (s *Slice) init() {
	for i := 0; i < len(s.dirty); i++ {
		if s.New == nil {
			// use queue
			s.dirty[i] = &LFQueue{}
		} else {
			s.dirty[i] = s.New()
		}
	}
	s.popID = s.pushID
	s.len = 0
}

// Init prevent push new element into queue
func (s *Slice) Init() {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()

	s.popMu.Lock()
	defer s.popMu.Unlock()

	s.init()
}

func (s *Slice) Size() int {
	return int(atomic.LoadUint32(&s.len))
}

func (s *Slice) Push(i interface{}) {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	s.onceInit()

	id := atomic.LoadUintptr(&s.pushID)
	s.hash(id).Push(i)
	atomic.AddUintptr(&s.pushID, 1)
	atomic.AddUint32(&s.len, 1)
}

func (s *Slice) Pop() interface{} {
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
	atomic.AddUint32(&s.len, negativeOne)
	return e
}

func casUintptr(addr *uintptr, old, new uintptr) bool {
	return atomic.CompareAndSwapUintptr(addr, old, new)
}
