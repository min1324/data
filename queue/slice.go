package queue

import (
	"sync/atomic"
)

// BUG

const (
	bit  = 3
	mod  = 1<<bit - 1
	null = ^uintptr(0) // -1
)

// Slice an array of queue
// each time have concurrent Add pushID
// but only one can get and add popID
// if popID == (pushID or 0),it means queue empty,
// return -1
type Slice struct {
	dirty  [mod + 1]Queue
	count  uintptr
	pushID uintptr
	popID  uintptr
}

func (s *Slice) hash(id uintptr) *Queue {
	return &s.dirty[id&mod]
}

// getPushID only use in Push
// because Push must success,
// so get the unique ID means push an item into queue
func (s *Slice) getPushID() uintptr {
	return atomic.AddUintptr(&s.pushID, 1)
}

func (s *Slice) getPopID() uintptr {
	for {
		popId := atomic.LoadUintptr(&s.popID)
		if popId == atomic.LoadUintptr(&s.pushID) {
			return null
		}
		newId := popId
		atomic.AddUintptr(&newId, 1)
		if atomic.CompareAndSwapUintptr(&s.popID, popId, newId) {
			return newId
		}
	}
}

func (s *Slice) Init() {

}

func (s *Slice) Size() int {
	return int(atomic.LoadUintptr(&s.count))
}

func (s *Slice) Push(i interface{}) {
	id := s.getPushID()
	s.hash(id).Push(i)
	atomic.AddUintptr(&s.count, 1)
}

func (s *Slice) Pop() interface{} {
	id := s.getPopID()
	if id == null {
		return nil
	}
	atomic.AddUintptr(&s.count, null)
	return s.hash(id).Pop()
}
