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
	q.data = make([]interface{}, 0, defaultQueueSize)
}

func (q *SAQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	q.init()
	// free queue [s ->...-> e]
}

func (q *SAQueue) Size() int {
	return len(q.data)
}

func (q *SAQueue) Push(i interface{}) {
	q.mu.Lock()
	q.onceInit()
	q.data = append(q.data, i)
	q.mu.Unlock()
}

func (q *SAQueue) Pop() interface{} {
	if q.Size() == 0 {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	if q.Size() == 0 {
		return nil
	}
	slot := q.data[0]
	q.data = q.data[1:]
	return slot
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
	data   []snode
}

type snode struct {
	val unsafe.Pointer
}

func (q *SRQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *SRQueue) init() {
	if q.cap < 1 {
		q.cap = defaultQueueSize
	}
	q.popID = q.pushID
	q.len = 0
	q.mod = minModU32(q.cap)
	q.cap = q.mod + 1
	q.data = make([]snode, q.cap)
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

func (q *SRQueue) Size() int {
	return int(q.len)
}

func (q *SRQueue) Full() bool {
	return q.pushID^q.cap == q.popID
}

func (q *SRQueue) Empty() bool {
	return q.popID == q.pushID
}

// 根据pushID,popID获取进队，出队对应的slot
func (q *SRQueue) getSlot(id uint32) *snode {
	return &q.data[int(id&q.mod)]
}

func (q *SRQueue) Set(i interface{}) bool {
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

func (n *snode) store(i interface{}) {
	atomic.StorePointer(&n.val, unsafe.Pointer(&i))
}

func (q *SRQueue) Get() (interface{}, bool) {
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
	val := slot.load()
	q.popID += 1
	q.len -= 1
	slot.free()
	return val, true
}

func (n *snode) load() interface{} {
	return *(*interface{})(n.val)
}

func (n *snode) free() {
}

func (q *SRQueue) Push(i interface{}) {
	for !q.Set(i) {
	}
}

func (q *SRQueue) Pop() interface{} {
	e, _ := q.Get()
	return e
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
	data   []dnode
}

type dnode struct {
	// val为空，表示可以push,如果时pop操作，表示队列空。
	// val不为空，表所可以pop,如果是push操作，表示队列满了。
	// 并且只能由push将val从nil变成非nil,
	// 只能由pop将val从非niu变成nil.
	val unsafe.Pointer
}

type drQueue *struct{}

// 表示空值
var drEmpty = unsafe.Pointer(new(interface{}))

func (q *DRQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *DRQueue) init() {
	if q.cap < 1 {
		q.cap = defaultQueueSize
	}
	q.popID = q.pushID
	q.len = 0
	q.mod = minModU32(q.cap)
	q.cap = q.mod + 1
	q.data = make([]dnode, q.cap)
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

func (q *DRQueue) Size() int {
	return int(q.len)
}

func (q *DRQueue) Full() bool {
	return q.pushID^q.cap == q.popID
}

func (q *DRQueue) Empty() bool {
	return q.popID == q.pushID
}

// 根据pushID,popID获取进队，出队对应的slot
func (q *DRQueue) getSlot(id uint32) *dnode {
	return &q.data[int(id&q.mod)]
}

func (q *DRQueue) Set(i interface{}) bool {
	q.pushMu.Lock()
	defer q.pushMu.Unlock()
	q.onceInit()

	slot := q.getSlot(q.pushID)
	if slot.val != nil {
		// 队列满了
		return false
	}
	atomic.AddUint32(&q.len, 1)
	atomic.AddUint32(&q.pushID, 1)
	if i == nil {
		i = drEmpty
	}
	slot.store(i)
	return true
}

func (n *dnode) store(i interface{}) {
	atomic.StorePointer(&n.val, unsafe.Pointer(&i))
}

func (q *DRQueue) Get() (interface{}, bool) {
	if q.Empty() {
		return nil, false
	}
	q.popMu.Lock()
	defer q.popMu.Unlock()
	q.onceInit()

	slot := q.getSlot(q.popID)
	if slot.val == nil {
		// 队列空了
		return nil, false
	}
	val := slot.load()
	if val == drEmpty {
		val = nil
	}
	atomic.AddUint32(&q.len, ^uint32(0))
	atomic.AddUint32(&q.popID, 1)
	slot.free()
	return val, true
}

func (n *dnode) load() interface{} {
	return *(*interface{})(n.val)
}

func (n *dnode) free() {
	n.val = nil
}

func (q *DRQueue) Push(i interface{}) {
	q.Set(i)
}

func (q *DRQueue) Pop() interface{} {
	e, _ := q.Get()
	return e
}

// ---------------------------	element in queue and stack		-----------------------------//

// element stack,queue element
type element struct {
	// p    unsafe.Pointer
	p    interface{}
	next *element
}

func newElement(i interface{}) *element {
	return &element{p: i}
	// return &element{p: unsafe.Pointer(&i)}
}

func (n *element) load() interface{} {
	return n.p
	//return *(*interface{})(n.p)
}

func (n *element) store(i interface{}) {
	n.p = i
	//return *(*interface{})(n.p)
}

// 释放当前element,n.next需要先保存好。
func (n *element) free() {
	n.next = nil
	n.p = nil
}

// ---------------------------		single mutex list queue		-----------------------------//

// SLQueue unbounded list queue with one mutex
type SLQueue struct {
	once sync.Once
	mu   sync.Mutex

	len  int
	head *element
	tail *element
}

func (q *SLQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *SLQueue) init() {
	q.head = &element{}
	q.tail = q.head
	q.len = 0
}

func (q *SLQueue) Init() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()

	head := q.head // start element
	tail := q.tail // end element
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

func (q *SLQueue) Size() int {
	return q.len
}

func (q *SLQueue) Push(i interface{}) {
	q.mu.Lock()
	q.onceInit()
	slot := newElement(i)
	q.tail.next = slot
	q.tail = slot
	q.len++
	q.mu.Unlock()
}

func (q *SLQueue) Pop() interface{} {
	if q.head == q.tail {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onceInit()
	if q.head.next == nil {
		return nil
	}
	slot := q.head
	q.head = q.head.next
	val := q.head.load()
	slot.free()
	q.len--
	return val
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

// element stack,queue element
type dlnode struct {
	// p    unsafe.Pointer
	p    interface{}
	next unsafe.Pointer
}

func newDLNode(i interface{}) *dlnode {
	return &dlnode{p: i}
	// return &dlnode{p: unsafe.Pointer(&i)}
}

func (n *dlnode) load() interface{} {
	return n.p
	//return *(*interface{})(n.p)
}

func (n *dlnode) store(i interface{}) {
	n.p = i
	//return *(*interface{})(n.p)
}

// 释放当前element,n.next需要先保存好。
func (n *dlnode) free() {
	n.next = nil
	n.p = nil
}

func (q *DLQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *DLQueue) init() {
	q.head = unsafe.Pointer(newDLNode(nil))
	q.tail = q.head
	q.len = 0
}

func (q *DLQueue) Init() {
	q.pushMu.Lock()
	defer q.pushMu.Unlock()
	q.popMu.Lock()
	defer q.popMu.Unlock()

	q.onceInit()
	head := q.head // start element
	tail := q.tail // end element
	if head == tail {
		return
	}
	q.head = q.tail
	q.len = 0
	// free queue [s ->...-> e]
	for head != tail && head != nil {
		node := (*dlnode)(head)
		head = node.next
		node.free()
	}
	return
}

func (q *DLQueue) Size() int {
	return int(q.len)
}

func (q *DLQueue) Push(i interface{}) {
	q.pushMu.Lock()
	defer q.pushMu.Unlock()
	q.onceInit()

	// 无满条件限制，直接执行。
	// tail只能由push更改，无竞争,pop只读
	// tail := atomic.LoadPointer(&q.tail)
	tail := q.tail
	tailNode := (*dlnode)(tail)

	slot := newDLNode(i)
	tailNode.next = unsafe.Pointer(slot)
	atomic.AddUint32(&q.len, 1)

	// 更新tail
	q.tail = unsafe.Pointer(slot)
	// atomic.StorePointer(&q.tail, unsafe.Pointer(slot))
}

func (q *DLQueue) Pop() interface{} {
	if q.head == q.tail {
		return nil
	}
	q.popMu.Lock()
	defer q.popMu.Unlock()
	q.onceInit()

	// head落后tail,即便tail更改了，出队操作无影响
	// 只需保证队列非空即可出队.
	if q.head == q.tail {
		// 队列空，返回
		return nil
	}
	headNode := (*dlnode)(q.head)
	slot := (*dlnode)(headNode.next)
	q.head = headNode.next
	val := slot.load()
	atomic.AddUint32(&q.len, ^uint32(0))
	headNode.free()
	return val
}

// 溢出环形计算需要，得出2^n-1。(2^n>=u,具体可见kfifo）
func minModU32(u uint32) uint32 {
	u -= 1 //兼容0, as min as ,128->127 !255
	u |= u >> 1
	u |= u >> 2
	u |= u >> 4
	u |= u >> 8  // 32位类型已经足够
	u |= u >> 16 // 64位
	return u
}
