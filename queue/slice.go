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

	len    uint32
	cap    uint32
	mod    uint32
	popID  uint32 // current pop id
	pushID uint32 // current push id

	state uint32 // 不为0时，初始化,无法执行push,pop.只能由Init更改

	dirty [mod + 1]Queue
	New   func() Queue
}

func (s *Slice) hash(id uint32) Queue {
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
			s.dirty[i] = &LLQueue{}
		} else {
			s.dirty[i] = s.New()
		}
	}
	if s.cap < 1 {
		s.cap = DefauleSize
	}
	s.popID = s.pushID
	s.mod = modUint32(s.cap)
	s.cap = s.mod + 1
	s.len = 0
}

// Init prevent push new element into queue
func (s *Slice) Init() {
	atomic.StoreUint32(&s.state, 1)
	s.init()
	atomic.StoreUint32(&s.state, 0)
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
		pushID := atomic.LoadUint32(&s.pushID)
		slot := s.hash(pushID)
		if casUint32(&s.pushID, pushID, pushID+1) {
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
		popPID := atomic.LoadUint32(&s.popID)
		if popPID == atomic.LoadUint32(&s.pushID) {
			return nil, false
		}
		slot := s.hash(popPID)
		if casUint32(&s.popID, popPID, popPID+1) {
			val, ok := slot.DeQueue()
			if val == nil && ok {
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
