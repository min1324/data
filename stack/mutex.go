package stack

import (
	"sync"
)

// mutex stack
// 单锁有限数组栈
type SAStack struct {
	once sync.Once
	mu   sync.Mutex

	len  uint32 // 栈数量，也指栈顶指向
	cap  uint32
	data []ptrNode
}

func (q *SAStack) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *SAStack) init() {
	if q.cap < 1 {
		q.cap = DefauleSize
	}
	q.len = 0
	q.data = make([]ptrNode, q.cap)
}

func (q *SAStack) Init() {
	q.InitWith()
}

// InitWith 初始化长度为cap的queue,
// 如果未提供，则使用默认值: DefaultSize
func (q *SAStack) InitWith(caps ...int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	if len(caps) > 0 && caps[0] > 0 {
		q.cap = uint32(caps[0])
	}
	q.init()
	for i := 0; i < len(q.data); i++ {
		q.data[i].free()
	}
}

func (q *SAStack) Cap() int {
	return int(q.cap)
}

func (q *SAStack) Full() bool {
	return q.len == q.cap
}

func (q *SAStack) Empty() bool {
	return q.len == 0
}

func (q *SAStack) Size() int {
	return int(q.len)
}

func (q *SAStack) Push(val interface{}) bool {
	if q.Full() {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.Full() {
		return false
	}
	if val == nil {
		val = empty
	}

	q.data[q.len].store(val)
	q.len += 1
	return true
}

func (q *SAStack) Pop() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.Empty() {
		return
	}
	slot := q.data[q.len-1]
	q.len -= 1
	val = slot.load()
	if val == empty {
		val = nil
	}
	slot.free()
	return val, true
}

// mutex list stack

// 单锁无限制链表栈
type SLStack struct {
	once sync.Once
	mu   sync.Mutex

	len uint32
	top *listNode
}

func (q *SLStack) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *SLStack) init() {
	top := q.top
	q.top = nil
	q.len = 0
	for top != nil {
		freeNode := top
		top = freeNode.next
		freeNode.free()
	}
}

func (q *SLStack) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.init()
}

func (q *SLStack) Full() bool {
	return false
}

func (q *SLStack) Empty() bool {
	return q.len == 0
}

func (q *SLStack) Size() int {
	return int(q.len)
}

func (q *SLStack) Push(val interface{}) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if val == nil {
		val = empty
	}
	slot := newListNode(val)
	slot.next = q.top
	q.top = slot
	q.len += 1
	return true
}

func (q *SLStack) Pop() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.Empty() {
		return
	}
	slot := q.top
	q.top = slot.next
	q.len -= 1
	val = slot.load()
	if val == empty {
		val = nil
	}
	slot.free()
	return val, true
}
