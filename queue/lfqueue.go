// lock-free queue package

package queue

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// LLQueue is a lock-free unbounded linked list queue.
type LLQueue struct {
	// 声明队列后，如果没有调用Init(),队列不能使用
	// 为解决这个问题，加入一次性操作。
	// 能在队列声明后，调用EnQueue或者DeQueue操作前，初始化队列。
	// 见 onceInit()
	once sync.Once

	// len is num of value store in queue
	len uint32

	// head指向第一个取数据的node。tail可能指向队尾元素。
	//
	// 出队操作，先检测head==tail判断队列是否空。
	// slot指向head先标记需要出队的数据。
	// 如果slot的val是空，表明队列空。
	// 然后通过cas将head指针指向slot.next以移除slot。
	// 释放slot.
	//
	// 入队操作，由于是链表队列，大小无限制，
	// 队列无满条件或者直到用完内存。不用判断是否满。
	// 让tail指向最后一个node,如果不是，提升tail,直到tail指向最后一个。
	// slot指向最后一个node,即slot=tail
	// 通过cas加入一个空node在slot.next后面
	// 将val存在slot里面
	//
	// head只能在DeQueue里面修改，tail只能在Enqueue修改。
	head unsafe.Pointer
	tail unsafe.Pointer
}

func (q *LLQueue) Size() int {
	return int(atomic.LoadUint32(&q.len))
}

// 一次性初始化,线程安全。
func (q *LLQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *LLQueue) init() {
	q.head = unsafe.Pointer(newPrtNode(nil))
	q.tail = q.head
	q.len = 0
}

func (q *LLQueue) Init() {
	q.onceInit()
	for {
		head := q.head
		tail := q.tail
		if head == tail {
			//  空队列不需要初始化
			return
		}
		// 置空队列
		oldLen := atomic.LoadUint32(&q.len)
		if cas(&q.head, head, tail) {
			// cas成功，表明期间并无其他操作，队列已经空了，但是len还没置0
			// 这里可能有并发EnQueue,增加len；即使有DeQueue,也能保持newLen>=oldLen
			for {
				newLen := atomic.LoadUint32(&q.len)
				detla := newLen - oldLen
				if atomic.CompareAndSwapUint32(&q.len, newLen, detla) {
					break
				}
			}

			for head != tail && head != nil {
				freeNode := (*ptrNode)(head)
				head = freeNode.next
				freeNode.free()
			}
			return
		}
	}
}

func (q *LLQueue) EnQueue(val interface{}) bool {
	q.onceInit()
	if val == nil {
		val = empty
	}
	// slot = tail,在slot.next插入一个空位，
	// 然后移动tail到空位,将数据保存在slot中
	nilNode := unsafe.Pointer(newPrtNode(nil))
	for {
		// 先取一下尾指针和尾指针的next
		tail := atomic.LoadPointer(&q.tail)
		slot := (*ptrNode)(tail)

		// 如果尾指针已经被移动了，则重新开始
		if tail != atomic.LoadPointer(&q.tail) {
			continue
		}

		// 如果尾指针的next!=nil，则提升tail直到指向最后一个位置
		if slot.next != nil {
			cas(&q.tail, tail, slot.next)
			continue
		}

		// next==nil,确定是最后一个
		if cas(&slot.next, nil, nilNode) {
			// 已经成功加入节点，尝试将 tail 提升到最新。
			cas(&q.tail, tail, nilNode)
			atomic.AddUint32(&q.len, 1)
			// 完成添加，将slot设置为可用，让等待的DeQueue可以取走
			slot.store(val)
			break
		}
	}
	return true
}

func (q *LLQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.onceInit()
	for {
		//取出头指针，尾指针，和第一个node指针
		head := atomic.LoadPointer(&q.head)
		tail := atomic.LoadPointer(&q.tail)
		slot := (*ptrNode)(head)

		// Q->head 其他DeQueue成功获得slot.指针已移动，重新取 head指针
		if head != atomic.LoadPointer(&q.head) {
			continue
		}

		if head == tail {
			// 即便tail落后了，也不提升。
			// tail只能在EnQueue里改变
			// return nil, false
			if slot.next == nil {
				return nil, false
			}
			cas(&q.tail, tail, slot.next)
			continue
		}
		// 先记录slot，然后尝试取出
		val := slot.load()
		if val == nil {
			// Enqueue还没添加完成，直接退出.如需等待，用continue
			return nil, false
		}
		if cas(&q.head, head, slot.next) {
			// 成功取出slot
			atomic.AddUint32(&q.len, negativeOne)
			if val == empty {
				val = nil
			}
			slot.free()
			return val, true
		}
	}
}

func (q *LLQueue) Full() bool {
	return false
}

func (q *LLQueue) Empty() bool {
	return q.head == q.tail
}

// range用于调试
func (q *LLQueue) Range(f func(interface{})) {
	if q.Empty() {
		return
	}

	head := q.head
	tail := q.tail
	for head != tail && head != nil {
		headNode := (*ptrNode)(head)
		n := (*ptrNode)(headNode.next)
		f(n.load())
		head = headNode.next
	}
}

// LLQueueUnsafe is a lock-free unbounded linked list queue.
type LLQueueUnsafe struct {
	// 声明队列后，如果没有调用Init(),队列不能使用
	// 为解决这个问题，加入一次性操作。
	// 能在队列声明后，调用EnQueue或者DeQueue操作前，初始化队列。
	// 见 onceInit()
	once sync.Once

	// len is num of value store in queue
	len uint32

	// head指向第一个取数据的node。tail可能指向队尾元素。
	//
	// 出队操作，先检测head==tail判断队列是否空。
	// slot指向head先标记需要出队的数据。
	// 如果slot的val是空，表明队列空。
	// 然后通过cas将head指针指向slot.next以移除slot。
	// 释放slot.
	//
	// 入队操作，由于是链表队列，大小无限制，
	// 队列无满条件或者直到用完内存。不用判断是否满。
	// 让tail指向最后一个node,如果不是，提升tail,直到tail指向最后一个。
	// slot指向最后一个node,即slot=tail
	// 通过cas加入一个空node在slot.next后面
	// 将val存在slot里面
	//
	// head只能在DeQueue里面修改，tail只能在Enqueue修改。
	head unsafe.Pointer
	tail unsafe.Pointer
}

func (q *LLQueueUnsafe) Size() int {
	return int(atomic.LoadUint32(&q.len))
}

// 一次性初始化,线程安全。
func (q *LLQueueUnsafe) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *LLQueueUnsafe) init() {
	q.head = unsafe.Pointer(newUnListNode(nil))
	q.tail = q.head
	q.len = 0
}

func (q *LLQueueUnsafe) Init() {
	q.onceInit()
	for {
		head := q.head
		tail := q.tail
		if head == tail {
			//  空队列不需要初始化
			return
		}
		// 置空队列
		oldLen := atomic.LoadUint32(&q.len)
		if cas(&q.head, head, tail) {
			// cas成功，表明期间并无其他操作，队列已经空了，但是len还没置0
			// 这里可能有并发EnQueue,增加len；即使有DeQueue,也能保持newLen>=oldLen
			for {
				newLen := atomic.LoadUint32(&q.len)
				detla := newLen - oldLen
				if atomic.CompareAndSwapUint32(&q.len, newLen, detla) {
					break
				}
			}

			for head != tail && head != nil {
				freeNode := (*unListNode)(head)
				head = freeNode.next
				freeNode.free()
			}
			return
		}
	}
}

func (q *LLQueueUnsafe) EnQueue(val interface{}) bool {
	q.onceInit()
	if val == nil {
		val = empty
	}
	// slot = tail,在slot.next插入一个空位，
	// 然后移动tail到空位,将数据保存在slot中
	nilNode := unsafe.Pointer(newUnListNode(nil))
	for {
		// 先取一下尾指针和尾指针的next
		tail := atomic.LoadPointer(&q.tail)
		slot := (*unListNode)(tail)

		// 如果尾指针已经被移动了，则重新开始
		if tail != atomic.LoadPointer(&q.tail) {
			continue
		}

		// 如果尾指针的next!=nil，则提升tail直到指向最后一个位置
		if slot.next != nil {
			cas(&q.tail, tail, slot.next)
			continue
		}

		// next==nil,确定是最后一个
		if cas(&slot.next, nil, nilNode) {
			// 已经成功加入节点，尝试将 tail 提升到最新。
			cas(&q.tail, tail, nilNode)
			atomic.AddUint32(&q.len, 1)
			// 完成添加，将slot设置为可用，让等待的DeQueue可以取走
			slot.store(val)
			break
		}
	}
	return true
}

func (q *LLQueueUnsafe) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.onceInit()
	for {
		//取出头指针，尾指针，和第一个node指针
		head := atomic.LoadPointer(&q.head)
		tail := atomic.LoadPointer(&q.tail)
		slot := (*unListNode)(head)

		// Q->head 其他DeQueue成功获得slot.指针已移动，重新取 head指针
		if head != atomic.LoadPointer(&q.head) {
			continue
		}

		if head == tail {
			// 即便tail落后了，也不提升。
			// tail只能在EnQueue里改变
			// return nil, false
			if slot.next == nil {
				return nil, false
			}
			cas(&q.tail, tail, slot.next)
			continue
		}
		// 先记录slot，然后尝试取出
		val := slot.load()
		if val == nil {
			// Enqueue还没添加完成，直接退出.如需等待，用continue
			return nil, false
		}
		if cas(&q.head, head, slot.next) {
			// 成功取出slot
			atomic.AddUint32(&q.len, negativeOne)
			if val == empty {
				val = nil
			}
			slot.free()
			return val, true
		}
	}
}

func (q *LLQueueUnsafe) Full() bool {
	return false
}

func (q *LLQueueUnsafe) Empty() bool {
	return q.head == q.tail
}

// lock-free queue implement with array
//
// LRQueue is a lock-free ring array queue.
type LRQueue struct {
	once sync.Once

	len  uint32 // 队列当前数据长度
	cap  uint32 // 队列容量，自动向上调整至2^n
	mod  uint32 // cap-1,即2^n-1,用作取slot: data[ID&mod]
	deID uint32 // 指向下次取出数据的位置:deID&mod
	enID uint32 // 指向下次写入数据的位置:enID&mod

	// 环形队列，大小必须是2的倍数。
	// val为空，表示可以EnQUeue,如果是DeQueue操作，表示队列空。
	// val不为空，表所可以DeQueue,如果是EnQUeue操作，表示队列满了。
	// 并且只能由EnQUeue将val从nil变成非nil,
	// 只能由DeQueue将val从非niu变成nil.
	data []baseNode
}

// 一次性初始化
func (q *LRQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *LRQueue) init() {
	if q.cap < 1 {
		q.cap = DefauleSize
	}
	q.mod = modUint32(q.cap)
	q.cap = q.mod + 1
	// 先清空data,dequeue无法减少len,
	// oldLen := atomic.LoadUint32(&q.len)

	q.data = make([]baseNode, q.cap)
	q.deID = q.enID
	q.len = 0
	// for {
	// 	newLen := atomic.LoadUint32(&q.len)
	// 	if casUint32(&q.len, newLen, newLen-oldLen) {
	// 		break
	// 	}
	// }

}

// Init初始化长度为: DefauleSize.
func (q *LRQueue) Init() {
	q.InitWith()
}

// InitWith初始化长度为cap的queue,
// 如果未提供，则使用默认值: DefauleSize.
// 线程不安全
func (q *LRQueue) InitWith(cap ...int) {
	if len(cap) > 0 && cap[0] > 0 {
		q.cap = uint32(cap[0])
	}
	q.init()
}

// 数量
func (q *LRQueue) Size() int {
	return int(q.len)
}

// 根据enID,deID获取进队，出队对应的slot
func (q *LRQueue) getSlot(id uint32) node {
	return &q.data[id&q.mod]
}

func (q *LRQueue) EnQueue(val interface{}) bool {
	if q.Full() {
		return false
	}
	q.onceInit()
	if val == nil {
		val = empty
	}
	for {
		enID := atomic.LoadUint32(&q.enID)
		slot := q.getSlot(enID)
		if slot.load() != nil {
			// TODO 是否需要写入缓冲区,或者扩容
			// queue full,
			return false
		}
		if casUint32(&q.enID, enID, enID+1) {
			// 成功获得slot
			atomic.AddUint32(&q.len, 1)
			slot.store(val)
			break
		}
	}
	return true
}

func (q *LRQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.onceInit()
	for {
		// 获取最新 DeQueuePID,
		deID := atomic.LoadUint32(&q.deID)
		slot := q.getSlot(deID)
		if slot.load() == nil {
			// queue empty,
			return nil, false
		}
		if casUint32(&q.deID, deID, deID+1) {
			// 成功取出slot
			atomic.AddUint32(&q.len, negativeOne)
			val = slot.load()
			if val == empty {
				val = nil
			}
			break
		}
	}
	return val, true
}

// queue's len
func (q *LRQueue) Len() int {
	return int(q.len)
}

// queue's cap
func (q *LRQueue) Cap() int {
	return int(q.cap)
}

// 队列是否满
func (q *LRQueue) Full() bool {
	return q.enID^q.cap == q.deID
	// return atomic.LoadUintptr(&q.len) == atomic.LoadUintptr(&q.cap)
	// return (q.deID&q.mod + 1) == (q.enID & q.mod)
}

// 队列是否空
func (q *LRQueue) Empty() bool {
	// return atomic.LoadUintptr(&q.len) == 0
	return q.deID == q.enID
}

var (
	errTimeOut = errors.New("超时")
)

// 带超时EnQueue入队。
func (q *LRQueue) PutWait(i interface{}, timeout time.Duration) (bool, error) {
	t := time.NewTicker(timeout)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			return false, errTimeOut
		default:
			if q.EnQueue(i) {
				return true, nil
			}
		}
	}
}

// not use yet
func (q *LRQueue) grow() bool {
	// TODO grow queue data
	return false
}
