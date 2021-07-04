package queue

import (
	"sync"
	"sync/atomic"
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
}

func (q *SAQueue) Cap() int {
	return queueLimit
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
	if q.Full() {
		return false
	}
	q.mu.Lock()
	q.onceInit()
	q.data = append(q.data, i)
	q.mu.Unlock()
	return true
}

func (q *SAQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.Empty() {
		return
	}
	val = q.data[0]
	q.data = q.data[1:]
	return val, true
}

// ---------------------------		single mutex ring queue		-----------------------------//

// 单锁环形队列,有固定数组
// 游标采取先操作，后移动方案。
// EnQUeue,DeQueue操作时，先操作slot增改value
// 操作完成后移动deID,enID.
// 队列空条件为deID==enID
// 满条件enID^cap==deID
//
// SRQueue is an unbounded queue which uses a slice as underlying.
type SRQueue struct {
	once sync.Once
	mu   sync.Mutex

	len uint32
	cap uint32
	mod uint32

	deID uint32
	enID uint32

	data []baseNode
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
	q.mod = modUint32(q.cap)
	q.cap = q.mod + 1
	q.deID = q.enID
	q.data = make([]baseNode, q.cap)
	q.len = 0
}

func (q *SRQueue) Init() {
	q.InitWith()
}

// InitWith 初始化长度为cap的queue,
// 如果未提供，则使用默认值: DefaultSize
func (q *SRQueue) InitWith(caps ...int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if len(caps) > 0 && caps[0] > 0 {
		q.cap = uint32(caps[0])
	}
	q.init()
	for i := 0; i < cap(q.data); i++ {
		q.data[i].free()
	}
}

func (q *SRQueue) Cap() int {
	return int(q.cap)
}

func (q *SRQueue) Full() bool {
	return q.enID^q.cap == q.deID
}

func (q *SRQueue) Empty() bool {
	return q.deID == q.enID
}

func (q *SRQueue) Size() int {
	return int(q.len)
}

// 根据enID,deID获取进队，出队对应的slot
func (q *SRQueue) getSlot(id uint32) node {
	return &q.data[id&q.mod]
}

func (q *SRQueue) EnQueue(val interface{}) bool {
	if q.Full() {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.Full() {
		return false
	}
	q.getSlot(q.enID).store(val)
	q.enID += 1
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
	slot := q.getSlot(q.deID)
	val = slot.load()
	q.deID += 1
	q.len -= 1
	slot.free()
	return val, true
}

// ---------------------------		dobul mutex ring queue		-----------------------------//

// 双锁环形队列,有固定数组
// 游标采取先操作，后移动方案。
// EnQUeue,DeQueue操作时，先操作slot增改value
// 操作完成后移动deID,enID.
// 队列空条件为deID==enID
// 满条件enID^cap==deID
//
// DRQueue is an unbounded queue which uses a slice as underlying.
type DRQueue struct {
	once sync.Once
	deMu sync.Mutex
	enMu sync.Mutex

	len uint32
	cap uint32
	mod uint32

	enID uint32
	deID uint32

	// val为空，表示可以EnQUeue,如果是DeQueue操作，表示队列空。
	// val不为空，表所可以DeQueue,如果是EnQUeue操作，表示队列满了。
	// 并且只能由EnQUeue将val从nil变成非nil,
	// 只能由DeQueue将val从非niu变成nil.
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
	q.mod = modUint32(q.cap)
	q.cap = q.mod + 1
	q.deID = q.enID
	q.data = make([]baseNode, q.cap)
	q.len = 0
}

func (q *DRQueue) Init() {
	q.InitWith()
}

// InitWith 初始化长度为cap的queue,
// 如果未提供，则使用默认值: DefaultSize
func (q *DRQueue) InitWith(cap ...int) {
	q.enMu.Lock()
	defer q.enMu.Unlock()
	q.deMu.Lock()
	defer q.deMu.Unlock()
	q.onceInit()

	if len(cap) > 0 && cap[0] > 0 {
		q.cap = uint32(cap[0])
	}
	q.init()
}

func (q *DRQueue) Cap() int {
	return int(q.cap)
}

func (q *DRQueue) Full() bool {
	return q.enID^q.cap == q.deID
}

func (q *DRQueue) Empty() bool {
	return q.deID == q.enID
}

func (q *DRQueue) Size() int {
	return int(q.len)
}

// 根据enID,deID获取进队，出队对应的slot
func (q *DRQueue) getSlot(id uint32) node {
	return &q.data[int(id&q.mod)]
}

func (q *DRQueue) EnQueue(val interface{}) bool {
	if q.Full() {
		return false
	}
	q.enMu.Lock()
	defer q.enMu.Unlock()
	q.onceInit()

	if q.Full() {
		return false
	}
	slot := q.getSlot(q.enID)
	if slot.load() != nil {
		// 队列满了
		return false
	}
	atomic.AddUint32(&q.enID, 1)
	atomic.AddUint32(&q.len, 1)
	if val == nil {
		val = empty
	}
	slot.store(val)
	return true
}

func (q *DRQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return nil, false
	}
	q.deMu.Lock()
	defer q.deMu.Unlock()
	q.onceInit()
	if q.Empty() {
		return nil, false
	}
	slot := q.getSlot(q.deID)
	if slot.load() == nil {
		// EnQueue正在写入
		return nil, false
	}
	val = slot.load()
	if val == empty {
		val = nil
	}
	atomic.AddUint32(&q.len, ^uint32(0))
	atomic.AddUint32(&q.deID, 1)
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
	q.head = newListNode(nil)
	q.tail = q.head
	q.len = 0
}

func (q *SLQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	head := q.head
	tail := q.tail
	if head == tail {
		return
	}
	q.head = q.tail
	q.len = 0
	for head != tail && head != nil {
		freeNode := head
		head = freeNode.next
		freeNode.free()
	}
	return
}

func (q *SLQueue) Cap() int {
	return queueLimit
}

func (q *SLQueue) Full() bool {
	return false
}

func (q *SLQueue) Empty() bool {
	return q.head == q.tail
}

func (q *SLQueue) Size() int {
	return q.len
}

func (q *SLQueue) EnQueue(val interface{}) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if val == nil {
		val = empty
	}
	// 方案1：tail指向最后一个有效node
	// slot := newListNode(i)
	// q.tail.next = slot
	// q.tail = slot

	// 方案2：tail指向下一个存入的空node
	slot := q.tail
	nilNode := newListNode(nil)
	slot.next = nilNode
	q.tail = nilNode
	slot.store(val)

	q.len++
	return true
}

func (q *SLQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.Empty() {
		return
	}
	// 方案1：head指向一个无效哨兵node
	// slot := q.head
	// q.head = q.head.next
	// val = q.head.load()

	// 方案2：head指向下一个取出的有效node
	slot := q.head
	val = slot.load()
	if val == nil {
		return
	}
	q.head = slot.next

	if val == empty {
		val = nil
	}
	q.len--
	slot.free()
	return val, true
}

// 双锁链表队列
//
// DLQueue is a concurrent unbounded queue which uses two-Lock concurrent queue qlgorithm.

// DLQueue unbounded list queue with one mutex
type DLQueue struct {
	once sync.Once
	deMu sync.Mutex // DeQueue操作锁
	enMu sync.Mutex // EnQUeue操作锁

	len  uint32
	head *listNode // 只能由DeQueue操作更改，其他操作只读
	tail *listNode // 只能由EnQUeue操作更改，其他操作只读
}

func (q *DLQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *DLQueue) init() {
	q.head = newListNode(nil)
	q.tail = q.head
	q.len = 0
}

func (q *DLQueue) Init() {
	q.enMu.Lock()
	defer q.enMu.Unlock()
	q.deMu.Lock()
	defer q.deMu.Unlock()
	q.onceInit()

	head := q.head
	tail := q.tail
	if head == tail {
		return
	}
	q.head = q.tail
	q.len = 0
	for head != tail && head != nil {
		freeNode := head
		head = freeNode.next
		freeNode.free()
	}
	return
}

func (q *DLQueue) Cap() int {
	return queueLimit
}

func (q *DLQueue) Full() bool {
	return false
}

func (q *DLQueue) Empty() bool {
	return q.head == q.tail
}

func (q *DLQueue) Size() int {
	return int(q.len)
}

func (q *DLQueue) EnQueue(val interface{}) bool {
	q.enMu.Lock()
	defer q.enMu.Unlock()
	q.onceInit()
	if val == nil {
		val = empty
	}
	// tail指向下一个存入的位置
	slot := q.tail
	nilNode := newListNode(nil)
	slot.next = nilNode
	q.tail = nilNode
	slot.store(val)
	atomic.AddUint32(&q.len, 1)
	return true
}

func (q *DLQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.deMu.Lock()
	defer q.deMu.Unlock()
	q.onceInit()
	if q.Empty() {
		return
	}
	slot := q.head
	val = slot.load()
	if val == nil {
		return
	}
	q.head = slot.next
	if val == empty {
		val = nil
	}
	atomic.AddUint32(&q.len, negativeOne)
	slot.free()
	return val, true
}
