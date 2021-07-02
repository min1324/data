package stack

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// LLStack a lock-free concurrent FILO stack.
type LLStack struct {
	len uint32         // stack value num.
	top unsafe.Pointer // point to the latest value pushed.
}

func (q *LLStack) onceInit() {}

// Init initialize stack.
func (s *LLStack) Init() {
	top := atomic.LoadPointer(&s.top)
	for top != nil {
		top = atomic.LoadPointer(&s.top)
		oldLen := atomic.LoadUint32(&s.len)
		if cas(&s.top, top, nil) {
			atomic.AddUint32(&s.len, (^oldLen + 1))
			break
		}
	}
	// free dummy node
	for top != nil {
		freeNode := (*ptrNode)(top)
		top = freeNode.next
		freeNode.free()
	}
}

func (q *LLStack) Full() bool {
	return false
}

func (q *LLStack) Empty() bool {
	return q.len == 0
}

// Size stack element's number
func (s *LLStack) Size() int {
	return int(atomic.LoadUint32(&s.len))
}

// Push puts the given value at the top of the stack.
func (s *LLStack) Push(val interface{}) bool {
	if val == nil {
		val = empty
	}
	slot := newPrtNode(val)
	for {
		slot.next = atomic.LoadPointer(&s.top)
		if cas(&s.top, slot.next, unsafe.Pointer(slot)) {
			atomic.AddUint32(&s.len, 1)
			break
		}
	}
	return true
}

// Pop removes and returns the value at the top of the stack.
// It returns nil if the stack is empty.
func (s *LLStack) Pop() (val interface{}, ok bool) {
	var slot *ptrNode
	for {
		top := atomic.LoadPointer(&s.top)
		if top == nil {
			return
		}
		slot = (*ptrNode)(top)
		if cas(&s.top, top, slot.next) {
			atomic.AddUint32(&s.len, ^uint32(0))
			break
		}
	}
	val = slot.load()
	if val == empty {
		val = nil
	}
	slot.free()
	return val, true
}

// LAStack a lock-free concurrent FILO array stack.
type LAStack struct {
	once sync.Once
	len  uint32
	cap  uint32
	data []listNode
}

// 一次性初始化
func (q *LAStack) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

// 无并发初始化
func (s *LAStack) init() {
	if s.cap < 1 {
		s.cap = DefauleSize
	}
	s.len = 0
	s.data = make([]listNode, s.cap)
}

// Init初始化长度为: DefauleSize.
func (s *LAStack) Init() {
	s.InitWith()
}

// InitWith初始化长度为cap的queue,
// 如果未提供，则使用默认值: DefauleSize.
func (s *LAStack) InitWith(caps ...int) {
	var cap = DefauleSize
	if len(caps) > 0 && caps[0] > 0 {
		cap = caps[0]
	}
	s.onceInit()
	s.len = s.cap + 1
	s.cap = 0
	s.data = make([]listNode, cap)
	s.len = 0
	s.cap = uint32(cap)
}

func (q *LAStack) Cap() int {
	return int(q.cap)
}

func (q *LAStack) Full() bool {
	return q.len == q.cap
}

func (q *LAStack) Empty() bool {
	return q.len == 0
}

// Size stack element's number
func (s *LAStack) Size() int {
	return int(s.len)
}

func (s *LAStack) getSlot(id uint32) node {
	return &s.data[id]
}

// Push puts the given value at the top of the stack.
func (s *LAStack) Push(val interface{}) bool {
	if val == nil {
		val = empty
	}
	for {
		top := atomic.LoadUint32(&s.len)
		if top >= s.cap {
			return false
		}
		slot := s.getSlot(top)
		if slot.load() != nil {
			continue
		}
		if casUint32(&s.len, top, top+1) {
			slot.store(val)
			break
		}
	}
	return true
}

// Pop removes and returns the value at the top of the stack.
// It returns nil if the stack is empty.
func (s *LAStack) Pop() (val interface{}, ok bool) {
	var slot node
	for {
		top := atomic.LoadUint32(&s.len)
		if top == 0 || top > s.cap {
			// top > s.cap withInit时缩短stack,肯能出现情况
			return
		}
		slot = s.getSlot(top - 1)
		val = slot.load()
		if val == nil {
			return
		}
		if casUint32(&s.len, top, top-1) {
			break
		}
	}
	if val == empty {
		val = nil
	}
	slot.free()
	return val, true
}
