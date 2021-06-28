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

type Interface interface {
	Init()
	Size() int
	Push(interface{})
	Pop() interface{}
}

// Slice an array of queue
type Slice struct {
	count  uintptr // size
	popID  uintptr // current pop id
	pushID uintptr // current push id

	dirty [mod + 1]Interface
	New   func() Interface

	pushMu sync.Mutex
	popMu  sync.Mutex
	once   sync.Once
}

func (s *Slice) hash(id uintptr) Interface {
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
			s.dirty[i] = &Queue{}
		} else {
			s.dirty[i] = s.New()
		}
	}
	s.popID = s.pushID
	s.count = 0
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
	return int(atomic.LoadUintptr(&s.count))
}

func (s *Slice) Push(i interface{}) {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	s.onceInit()

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

func casUintptr(addr *uintptr, old, new uintptr) bool {
	return atomic.CompareAndSwapUintptr(addr, old, new)
}
