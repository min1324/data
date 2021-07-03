package stack

import (
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

// // LAStack a lock-free concurrent FILO array stack.
// type LAStack struct {
// 	once  sync.Once
// 	len   uint32
// 	cap   uint32
// 	state uint32
// 	data  []stateNode
// }

// // 一次性初始化
// func (q *LAStack) onceInit() {
// 	q.once.Do(func() {
// 		q.init()
// 	})
// }

// // 无并发初始化
// func (s *LAStack) init() {
// 	var cap = s.cap
// 	if cap < 1 {
// 		cap = DefauleSize
// 	}
// 	s.data = make([]stateNode, cap)
// 	s.len = 0
// 	s.cap = cap
// }

// // Init初始化长度为: DefauleSize.
// func (s *LAStack) Init() {
// 	s.InitWith()
// }

// // InitWith初始化长度为cap的queue,
// // 如果未提供，则使用默认值: DefauleSize.
// func (s *LAStack) InitWith(caps ...int) {
// 	s.onceInit()
// 	// if s.cap < 1 {
// 	// 	// 第一次初始化，cap为0.
// 	// 	s.cap = DefauleSize
// 	// }
// 	var newCap = atomic.LoadUint32(&s.cap)
// 	// if len(caps) > 0 && caps[0] > 0 {
// 	// 	newCap = uint32(caps[0])
// 	// }
// 	// s.cap==0,让新的Push,Pop无法执行。
// 	for {
// 		oldCap := atomic.LoadUint32(&s.cap)
// 		if oldCap == 0 {
// 			// other InitWith running,wait until it finished.
// 			continue
// 		}
// 		if casUint32(&s.cap, oldCap, 0) {
// 			// 获得了初始化权限
// 			break
// 		}
// 	}
// 	// top == 0，让执行中的Push,Pop结束执行.
// 	for {
// 		top := atomic.LoadUint32(&s.len)
// 		if top == 0 {
// 			break
// 		}
// 		slot := s.getSlot(top - 1)
// 		if casUint32(&slot.state, 0, 1) {
// 			atomic.StoreUint32(&s.len, 0)
// 			slot.change(0)
// 			break
// 		}
// 	}
// 	// 初始化
// 	s.data = make([]stateNode, newCap)
// 	s.len = 0
// 	s.cap = newCap
// }

// func (s *LAStack) getSlot(id uint32) *stateNode {
// 	return &s.data[id]
// }

// // Push puts the given value at the top of the stack.
// func (s *LAStack) Push(val interface{}) bool {
// 	s.onceInit()
// 	if val == nil {
// 		val = empty
// 	}
// 	for {
// 		top := atomic.LoadUint32(&s.len)
// 		if top >= s.cap {
// 			return false
// 		}
// 		slot := s.getSlot(top)
// 		if casUint32(&slot.state, 0, 1) {
// 			slot.store(val)
// 			atomic.AddUint32(&s.len, 1)
// 			slot.change(0)
// 			break
// 		}
// 	}
// 	return true
// }

// // Pop removes and returns the value at the top of the stack.
// // It returns nil if the stack is empty.
// func (s *LAStack) Pop() (val interface{}, ok bool) {
// 	s.onceInit()
// 	var slot *stateNode
// 	for {
// 		top := atomic.LoadUint32(&s.len)
// 		if top == 0 {
// 			// top > s.cap withInit时缩短stack,肯能出现情况
// 			return
// 		}
// 		slot = s.getSlot(top - 1)
// 		if casUint32(&slot.state, 0, 1) {
// 			val = slot.load()
// 			slot.free()
// 			atomic.AddUint32(&s.len, ^uint32(0))
// 			slot.change(0)
// 			break
// 		}
// 	}
// 	if val == empty {
// 		val = nil
// 	}
// 	return val, true
// }

// func (q *LAStack) Cap() int {
// 	return int(q.cap)
// }

// func (q *LAStack) Full() bool {
// 	return q.len == q.cap
// }

// func (q *LAStack) Empty() bool {
// 	return q.len == 0
// }

// // Size stack element's number
// func (s *LAStack) Size() int {
// 	return int(s.len)
// }
