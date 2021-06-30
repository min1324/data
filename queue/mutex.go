package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// ---------------------------		queue with slice	-----------------------------//

// SAQueue is an unbounded queue which uses a slice as underlying.
type SAQueue struct {
	once sync.Once
	mu   sync.Mutex
	data []interface{}
}

func (q *SAQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *SAQueue) init() {
	q.data = make([]interface{}, 0, DefauleSize)
}

func (q *SAQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	q.init()
	// free queue [s ->...-> e]
}

func (q *SAQueue) Full() bool {
	return false
}

func (q *SAQueue) Empty() bool {
	return len(q.data) == 0
}

func (q *SAQueue) Size() int {
	return len(q.data)
}

func (q *SAQueue) EnQueue(i interface{}) bool {
	q.mu.Lock()
	q.onceInit()
	q.data = append(q.data, i)
	q.mu.Unlock()
	return true
}

func (q *SAQueue) DeQueue() (val interface{}, ok bool) {
	if q.Size() == 0 {
		return nil, false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	if q.Size() == 0 {
		return nil, false
	}
	val = q.data[0]
	q.data = q.data[1:]
	return val, true
}

// ---------------------------		single mutex ring queue		-----------------------------//

// 单锁环形队列,有固定数组
// 游标采取先操作，后移动方案。
// push,pop操作时，先操作slot增改value
// 操作完成后移动popID,pushID.
// 队列空条件为popID==pushID
// 满条件pushID^cap==popID
//
// SRQueue is an unbounded queue which uses a slice as underlying.
type SRQueue struct {
	once sync.Once
	mu   sync.Mutex

	len uint32
	cap uint32
	mod uint32

	pushID uint32
	popID  uint32
	data   []baseNode
}

func (q *SRQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *SRQueue) init() {
	if q.cap < 1 {
		q.cap = DefauleSize
	}
	q.popID = q.pushID
	q.len = 0
	q.mod = modUint32(q.cap)
	q.cap = q.mod + 1
	q.data = make([]baseNode, q.cap)
}

func (q *SRQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	q.init()
	// free queue [s ->...-> e]
}

// InitWith 初始化长度为cap的queue,
// 如果未提供，则使用默认值: 1<<8
func (q *SRQueue) InitWith(cap ...int) {
	if len(cap) > 0 && cap[0] > 0 {
		q.cap = uint32(cap[0])
	}
	q.init()
}

func (q *SRQueue) Full() bool {
	return q.pushID^q.cap == q.popID
}

func (q *SRQueue) Empty() bool {
	return q.popID == q.pushID
}

func (q *SRQueue) Size() int {
	return int(q.len)
}

// 根据pushID,popID获取进队，出队对应的slot
func (q *SRQueue) getSlot(id uint32) *baseNode {
	return &q.data[int(id&q.mod)]
}

func (q *SRQueue) EnQueue(i interface{}) bool {
	if q.Full() {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	if q.Full() {
		return false
	}
	q.getSlot(q.pushID).store(i)
	q.pushID += 1
	q.len += 1
	return true
}

func (q *SRQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return nil, false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	if q.Empty() {
		return nil, false
	}
	slot := q.getSlot(q.popID)
	val = slot.load()
	q.popID += 1
	q.len -= 1
	slot.free()
	return val, true
}

// ---------------------------		dobul mutex ring queue		-----------------------------//

// 双锁环形队列,有固定数组
// 游标采取先操作，后移动方案。
// push,pop操作时，先操作slot增改value
// 操作完成后移动popID,pushID.
// 队列空条件为popID==pushID
// 满条件pushID^cap==popID
//
// DRQueue is an unbounded queue which uses a slice as underlying.
type DRQueue struct {
	once   sync.Once
	popMu  sync.Mutex
	pushMu sync.Mutex

	len uint32
	cap uint32
	mod uint32

	pushID uint32
	popID  uint32

	// val为空，表示可以push,如果时pop操作，表示队列空。
	// val不为空，表所可以pop,如果是push操作，表示队列满了。
	// 并且只能由push将val从nil变成非nil,
	// 只能由pop将val从非niu变成nil.
	data []baseNode
}

func (q *DRQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *DRQueue) init() {
	if q.cap < 1 {
		q.cap = DefauleSize
	}
	q.popID = q.pushID
	q.len = 0
	q.mod = modUint32(q.cap)
	q.cap = q.mod + 1
	q.data = make([]baseNode, q.cap)
}

func (q *DRQueue) Init() {
	q.pushMu.Lock()
	defer q.pushMu.Unlock()

	q.popMu.Lock()
	defer q.popMu.Unlock()
	q.onceInit()

	q.init()
	// free queue [s ->...-> e]
}

// InitWith 初始化长度为cap的queue,
// 如果未提供，则使用默认值: 1<<8
func (q *DRQueue) InitWith(cap ...int) {
	if len(cap) > 0 && cap[0] > 0 {
		q.cap = uint32(cap[0])
	}
	q.init()
}

func (q *DRQueue) Full() bool {
	return q.pushID^q.cap == q.popID
}

func (q *DRQueue) Empty() bool {
	return q.popID == q.pushID
}

func (q *DRQueue) Size() int {
	return int(q.len)
}

// 根据pushID,popID获取进队，出队对应的slot
func (q *DRQueue) getSlot(id uint32) *baseNode {
	return &q.data[int(id&q.mod)]
}

func (q *DRQueue) EnQueue(i interface{}) bool {
	if q.Full() {
		return false
	}
	q.pushMu.Lock()
	defer q.pushMu.Unlock()
	q.onceInit()

	slot := q.getSlot(q.pushID)
	if slot.load() != nil {
		// 队列满了
		return false
	}
	atomic.AddUint32(&q.len, 1)
	atomic.AddUint32(&q.pushID, 1)
	if i == nil {
		i = empty
	}
	slot.store(i)
	return true
}

func (q *DRQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return nil, false
	}
	q.popMu.Lock()
	defer q.popMu.Unlock()
	q.onceInit()

	slot := q.getSlot(q.popID)
	if slot.load() == nil {
		// 队列空了
		return nil, false
	}
	val = slot.load()
	if val == empty {
		val = nil
	}
	atomic.AddUint32(&q.len, ^uint32(0))
	atomic.AddUint32(&q.popID, 1)
	slot.free()
	return val, true
}

// ---------------------------		single mutex list queue		-----------------------------//

// SLQueue unbounded list queue with one mutex
type SLQueue struct {
	once sync.Once
	mu   sync.Mutex

	len  int
	head *listNode
	tail *listNode
}

func (q *SLQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *SLQueue) init() {
	q.head = &listNode{}
	q.tail = q.head
	q.len = 0
}

func (q *SLQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	head := q.head // start listNode
	tail := q.tail // end listNode
	if head == tail {
		return
	}
	q.head = q.tail
	q.len = 0
	// free queue [s ->...-> e]
	for head != tail && head != nil {
		el := head
		head = el.next
		el.next = nil
	}
	return
}

func (q *SLQueue) Full() bool {
	return false
}

func (q *SLQueue) Empty() bool {
	return q.len == 0
}

func (q *SLQueue) Size() int {
	return q.len
}

func (q *SLQueue) EnQueue(i interface{}) bool {
	q.mu.Lock()
	q.onceInit()
	slot := newListNode(i)
	q.tail.next = slot
	q.tail = slot
	q.len++
	q.mu.Unlock()
	return true
}

func (q *SLQueue) DeQueue() (val interface{}, ok bool) {
	if q.head == q.tail {
		return nil, false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.head.next == nil {
		return nil, false
	}
	slot := q.head
	q.head = q.head.next
	val = q.head.load()
	slot.free()
	q.len--
	return val, true
}

// 双锁链表队列
// push只需保证修改tail是最后一步。
//
// DLQueue is a concurrent unbounded queue which uses two-Lock concurrent queue qlgorithm.
type DLQueue struct {
	once   sync.Once
	popMu  sync.Mutex // pop操作锁
	pushMu sync.Mutex // push操作锁

	len  uint32
	head unsafe.Pointer // 只能由pop操作更改，其他操作只读
	tail unsafe.Pointer // 只能由push操作更改，其他操作只读
}

func (q *DLQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *DLQueue) init() {
	q.head = unsafe.Pointer(newPrtNode(nil))
	q.tail = q.head
	q.len = 0
}

func (q *DLQueue) Init() {
	q.pushMu.Lock()
	defer q.pushMu.Unlock()
	q.popMu.Lock()
	defer q.popMu.Unlock()

	q.onceInit()
	head := q.head // start listNode
	tail := q.tail // end listNode
	if head == tail {
		return
	}
	q.head = q.tail
	q.len = 0
	// free queue [s ->...-> e]
	for head != tail && head != nil {
		node := (*ptrNode)(head)
		head = node.next
		node.free()
	}
	return
}

func (q *DLQueue) Full() bool {
	return false
}

func (q *DLQueue) Empty() bool {
	return q.len == 0
}

func (q *DLQueue) Size() int {
	return int(q.len)
}

func (q *DLQueue) EnQueue(i interface{}) bool {
	q.pushMu.Lock()
	defer q.pushMu.Unlock()
	q.onceInit()

	// 无满条件限制，直接执行。
	// tail只能由push更改，无竞争,pop只读
	// tail := atomic.LoadPointer(&q.tail)
	tail := q.tail
	tailNode := (*ptrNode)(tail)

	slot := newPrtNode(i)
	tailNode.next = unsafe.Pointer(slot)
	atomic.AddUint32(&q.len, 1)

	// 更新tail
	q.tail = unsafe.Pointer(slot)
	// atomic.StorePointer(&q.tail, unsafe.Pointer(slot))
	return true
}

func (q *DLQueue) DeQueue() (val interface{}, ok bool) {
	if q.head == q.tail {
		return nil, false
	}
	q.popMu.Lock()
	defer q.popMu.Unlock()
	q.onceInit()

	// head落后tail,即便tail更改了，出队操作无影响
	// 只需保证队列非空即可出队.
	if q.head == q.tail {
		// 队列空，返回
		return nil, false
	}
	headNode := (*ptrNode)(q.head)
	slot := (*ptrNode)(headNode.next)
	q.head = headNode.next
	val = slot.load()
	atomic.AddUint32(&q.len, ^uint32(0))
	headNode.free()
	return val, true
}
