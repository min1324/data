package queue

import (
	"sync"
	"sync/atomic"
)

// BUG

const (
	bit  = 3
	mod  = 1<<bit - 1
	null = ^uintptr(0) // -1
)

// Slice an array of queue
type Slice struct {
	count  uintptr // size
	popID  uintptr // current pop id
	pushID uintptr // current push id

	dirty [mod + 1]Queue

	pushMu sync.Mutex
	popMu  sync.Mutex
}

func (s *Slice) hash(id uintptr) *Queue {
	return &s.dirty[id&mod]
}

func (s *Slice) Init() {
	s.count = 0
	s.popID = 0
	s.pushID = 0

	for _, e := range s.dirty {
		e.Init()
	}
}

func (s *Slice) Size() int {
	return int(atomic.LoadUintptr(&s.count))
}

func (s *Slice) Push(i interface{}) {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()

	id := atomic.LoadUintptr(&s.pushID)
	s.hash(id).Push(i)
	atomic.AddUintptr(&s.pushID, 1)
	atomic.AddUintptr(&s.count, 1)
}

func (s *Slice) Pop() interface{} {
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

func casUintptr(addr *uintptr, old, new uintptr) bool {
	return atomic.CompareAndSwapUintptr(addr, old, new)
}
