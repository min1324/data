// tishi package is a slice queue interface
// it has an array of [1<<bit]Queue

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
	once sync.Once

	len  uint32
	cap  uint32
	mod  uint32
	deID uint32
	enID uint32

	state uint32 // 不为0时，初始化,无法执行EnQueue,DeQueue.只能由Init更改

	data [mod + 1]DataQueue
	New  func() DataQueue
}

func (s *Slice) getSlot(id uint32) DataQueue {
	return s.data[id&mod]
}

func (s *Slice) onceInit() {
	s.once.Do(func() {
		s.init()
	})
}

func (s *Slice) init() {
	for i := 0; i < len(s.data); i++ {
		if s.New == nil {
			// use default lock-free queue
			s.data[i] = &LRQueue{}
		} else {
			s.data[i] = s.New()
		}
		s.data[i].onceInit()
	}
	if s.cap < 1 {
		s.cap = DefauleSize
	}
	s.deID = s.enID
	s.mod = modUint32(s.cap)
	s.cap = s.mod + 1
	s.len = 0
}

// Init prevent push new element into queue
func (s *Slice) Init() {
	s.onceInit()
	for {
		if atomic.CompareAndSwapUint32(&s.state, 0, 1) {
			s.init()
			atomic.StoreUint32(&s.state, 0)
			break
		}
	}
}

func (q *Slice) Full() bool {
	q.onceInit()
	for i := range q.data {
		if !q.data[i].Full() {
			return false
		}
	}
	return true
}

func (q *Slice) Empty() bool {
	return q.len == 0
}

func (s *Slice) Size() int {
	return int(atomic.LoadUint32(&s.len))
}

func (s *Slice) EnQueue(val interface{}) bool {
	s.onceInit()
	if val == nil {
		val = empty
	}
	for {
		if atomic.LoadUint32(&s.state) != 0 {
			// Init 执行中，无法操作
			continue
		}
		enID := atomic.LoadUint32(&s.enID)
		slot := s.getSlot(enID)
		if casUint32(&s.enID, enID, enID+1) {
			ok := slot.EnQueue(val)
			if ok {
				atomic.AddUint32(&s.len, 1)
			}
			return ok
		}
	}
}

func (s *Slice) DeQueue() (val interface{}, ok bool) {
	s.onceInit()
	for {
		if atomic.LoadUint32(&s.state) != 0 {
			// Init 执行中，无法操作
			continue
		}
		deID := atomic.LoadUint32(&s.deID)
		if deID == atomic.LoadUint32(&s.enID) {
			return nil, false
		}
		slot := s.getSlot(deID)
		if casUint32(&s.deID, deID, deID+1) {
			val, ok := slot.DeQueue()
			if val == nil || !ok {
				return nil, false
			}
			if val == empty {
				val = nil
			}
			atomic.AddUint32(&s.len, negativeOne)
			return val, true
		}
	}
}
