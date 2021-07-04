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

func (q *LLStack) Cap() int {
	return stackLimit
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

	// 高16位用来表示当前读线程数量，
	// 低16位表示当前写线程数量。
	// 允许连续读，连续写，但不允许读写同时进行。
	state uint32
	data  []baseNode
}

const stackBits = 16
const stackMark = (1 << stackBits) - 1
const stackLimit = (1 << 32) / 4

func (s LAStack) unpack(ptrs uint32) (read, write uint16) {
	read = uint16((ptrs >> stackBits) & stackMark)
	write = uint16(ptrs & stackMark)
	return
}

func (s LAStack) pack(read, write uint16) (ptrs uint32) {
	return (uint32(read)<<stackBits | uint32(write&stackMark))
}

// 一次性初始化
func (q *LAStack) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

// 无并发初始化
func (s *LAStack) init() {
	var cap = s.cap
	if cap < 1 {
		cap = DefauleSize
	}
	s.data = make([]baseNode, cap)
	s.len = 0
	s.state = 0
	s.cap = cap
}

// Init初始化长度为: DefauleSize.
func (s *LAStack) Init() {
	s.InitWith()
}

// InitWith初始化长度为cap的queue,
// 如果未提供，则使用默认值: DefauleSize.
func (s *LAStack) InitWith(caps ...int) {
	s.onceInit()
	if s.cap < 1 {
		// 第一次初始化，cap为0.
		s.cap = DefauleSize
	}
	var newCap = atomic.LoadUint32(&s.cap)
	if len(caps) > 0 && caps[0] > 0 {
		newCap = uint32(caps[0])
	}
	// s.cap==0,让新的Push,Pop无法执行。
	for {
		oldCap := atomic.LoadUint32(&s.cap)
		if oldCap == 0 {
			// other InitWith running,wait until it finished.
			continue
		}
		if casUint32(&s.cap, oldCap, 0) {
			// 获得了初始化权限
			break
		}
	}
	// 让执行中的Push,Pop结束执行.
	for {
		state := atomic.LoadUint32(&s.state)
		read, write := s.unpack(state)
		if read > 0 {
			// 有读线程，等待。
			continue
		}
		if write > 0 {
			// 写线程已经达到 1<<bit-1了，无法再添加。
			continue
		}
		stop := s.pack(stackMark, stackMark)
		if casUint32(&s.state, state, stop) {
			break
		}
	}
	// 初始化
	s.data = make([]baseNode, newCap)
	s.len = 0
	s.cap = newCap
	atomic.StoreUint32(&s.state, 0)
}

func (s *LAStack) getSlot(id uint32) *baseNode {
	return &s.data[id]
}

// Push puts the given value at the top of the stack.
func (s *LAStack) Push(val interface{}) bool {
	s.onceInit()
	if s.Full() {
		return false
	}
	if val == nil {
		val = empty
	}
	// 先增加写状态数量
	for {
		state := atomic.LoadUint32(&s.state)
		read, write := s.unpack(state)
		if read > 0 {
			// 有读线程，等待。
			continue
		}
		if write == stackMark {
			// 写线程已经达到 1<<bit-1了，无法再添加。
			continue
		}
		state2 := s.pack(0, write+1)
		if casUint32(&s.state, state, state2) {
			// 成功添加1个写线程
			break
		}
	}
	// 写线程已增加1，最后需要减少1
	defer atomic.AddUint32(&s.state, negativeOne)

	// 再获取slot
	var slot *baseNode
	for {
		top := atomic.LoadUint32(&s.len)
		if top >= s.cap {
			// withInit时缩短stack,肯能出现情况：top > s.cap，
			return false
		}
		slot = s.getSlot(top)
		if casUint32(&s.len, top, top+1) {
			break
		}
	}
	slot.store(val)
	return true
}

// Pop removes and returns the value at the top of the stack.
// It returns nil if the stack is empty.
func (s *LAStack) Pop() (val interface{}, ok bool) {
	s.onceInit()
	if s.Empty() {
		return
	}
	// 先增加读状态数量
	for {
		state := atomic.LoadUint32(&s.state)
		read, write := s.unpack(state)
		if write > 0 {
			// 有写线程，等待。
			continue
		}
		if read == stackMark {
			// 读线程已经达到 1<<bit-1了，无法再添加。
			continue
		}
		state2 := s.pack(read+1, 0)
		if casUint32(&s.state, state, state2) {
			// 成功添加1个读线程
			break
		}
	}
	// 读线程已增加1，最后需要减少1
	defer atomic.AddUint32(&s.state, ^uint32(stackMark))

	// 再获取slot
	var slot *baseNode
	for {
		top := atomic.LoadUint32(&s.len)
		if top == 0 || top > s.cap {
			// withInit时缩短stack,肯能出现情况：top > s.cap，
			return
		}
		slot = s.getSlot(top - 1)
		if casUint32(&s.len, top, top-1) {
			break
		}
	}
	val = slot.load()
	slot.free()
	if val == empty {
		val = nil
	}
	return val, true
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
