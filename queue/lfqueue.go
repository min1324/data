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

	// head指向哨兵位，不存数据。tail可能指向队尾元素。
	//
	// 出队操作，先检测head==tail判断队列是否空。
	// slot指向 head.next 先标记需要出队的数据。
	// 然后通过cas将head指针指向slot以获得出队权限。
	// 释放旧head.
	//
	// 入队操作，由于是链表队列，大小无限制，
	// 队列无满条件或者直到用完内存。不用判断是否满。
	// 先判断tail是否指向最后一个，通过:tail.next==nil
	// 如果不是，则需要让tail指向下一个来提升tail,直到tail指向最后一个。
	// 通过cas将slot加入tail.
	//
	// stat只有EnQueueed和DeQueueed状态。
	head unsafe.Pointer
	tail unsafe.Pointer
}

// Size queue element's number
func (q *LLQueue) Size() int {
	return int(atomic.LoadUint32(&q.len))
}

// 一次性初始化,线程安全。
func (q *LLQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

// 线程不安全初始化，只在声明队列后，
// 使用EnQueue或者DeQueue前调用一次。
func (q *LLQueue) init() {
	q.head = unsafe.Pointer(&stateNode{})
	q.tail = q.head
	q.len = 0
}

// Init initialize queue
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

		// 让head指向tail,置空队列，但是len并未置空，
		//
		// 因为置空队列，然后len=0两个操作期间，其他线程可能EnQueue。
		// I为Init，P为Push，运行路线可能：I将head指向tail,P通过EnQueue成功加入数据，len增加1，I将len置0.与预期1不相符。
		// 所以len无法直接置0。
		//
		// 如果先置len=0,再cas置空队列。
		// I为Init，P为Pop，运行路线可能：P将head指向 head.next,I将len置0.P将len-1(即是-1)，与预期0不相符。
		//
		// 两种方案：释放旧node时减少len,或在一次性更新len
		// 期间调用Size()无法获得正确的长度。

		// 方案1,一次性更新len
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
				freeNode := (*stateNode)(head)
				head = freeNode.next
				freeNode.free()
			}
			return
		}

		// 方案2,释放旧node,len-1
		if cas(&q.head, head, tail) {
			for head != tail && head != nil {
				freeNode := (*stateNode)(head)
				head = freeNode.next
				freeNode.free()
				atomic.AddUint32(&q.len, negativeOne)
			}
			return
		}
	}
}

func (q *LLQueue) EnQueue(val interface{}) bool {
	q.onceInit()
	slot := newStateNode(val)
	slotPtr := unsafe.Pointer(slot)
	for {
		// 先取一下尾指针和尾指针的next
		tail := atomic.LoadPointer(&q.tail)
		tailNode := (*stateNode)(tail)
		tailNext := tailNode.next

		// 如果尾指针已经被移动了，则重新开始
		if tail != atomic.LoadPointer(&q.tail) {
			continue
		}

		// 如果尾指针的next!=nil，则提升tail直到指向最后一个位置
		if tailNext != nil {
			cas(&q.tail, tail, tailNext)
			continue
		}

		// next==nil,确定是最后一个
		if cas(&tailNode.next, tailNext, slotPtr) {
			// 已经成功加入节点，尝试将 tail 提升到最新。
			cas(&q.tail, tail, slotPtr)
			// 更新len
			atomic.AddUint32(&q.len, 1)
			// 完成添加，将slot设置为可用，让等待的DeQueue可以取走
			slot.change(seted_stat)
			break
		}
	}
	return true
}

// Pop removes and returns the value at the head of the queue.
// It returns nil if the queue is empty.
func (q *LLQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.onceInit()
	for {
		//取出头指针，尾指针，和第一个node指针
		head := atomic.LoadPointer(&q.head)
		tail := atomic.LoadPointer(&q.tail)
		headNode := (*stateNode)(head)
		headNext := headNode.next

		// Q->head 其他DeQueue成功获得slot.指针已移动，重新取 head指针
		if head != atomic.LoadPointer(&q.head) {
			continue
		}

		if head == tail {
			// 队列可能空，可能tail落后。
			if headNext == nil {
				// 空队列返回
				return nil, false
			}
			// 此时head,tail指向同一个位置，并且有新增node
			// tail指针落后了,需要提升tail到下一个node,(tailNext==headNext)
			cas(&q.tail, tail, headNext)
			continue
		}
		// 取出slot

		// 先记录slot，然后尝试取出
		slot := (*stateNode)(headNext)
		if slot.status() != seted_stat {
			// TODO
			// EnQueue还没添加完成。
			// 方案1：直接返回nil。
			return nil, false

			// 方案2：等待EnQueue添加完成
			// continue
		}

		// 记录值，再尝试获取slot.
		// 如果先获取slot,再记录值。会出现的问题：
		// DeQueue1通过cas获取slot后暂停。DeQueue2获取到slot.next,调用headNode.free()，将slot释放掉
		// 此时slot已经被清空，获取到nil值。与预期不符。
		val := slot.load()
		if cas(&q.head, head, headNext) {
			// 成功取出slot,此时的slot可能被其他DeQueue释放。
			// len-1
			atomic.AddUint32(&q.len, negativeOne)

			// 释放head指向上次DeQueue操作的slot.
			headNode.free()
			return val, true
		}
	}
}

func (q *LLQueue) Full() bool {
	return false
}

func (q *LLQueue) Empty() bool {
	return atomic.LoadUint32(&q.len) == 0
}

// range用于调试
func (q *LLQueue) Range(f func(interface{})) {
	head := q.head
	tail := q.tail
	if head == tail {
		return
	}

	for head != tail && head != nil {
		headNode := (*stateNode)(head)
		n := (*stateNode)(headNode.next)
		f(n.load())
		head = headNode.next
	}
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
	// state有4个状态。
	// 这两个状态在DeQueue操作时都表示为队列空。
	// geted_stat 状态时，已经取出，可以写入。EnQueue操作通过cas改变为setting_stat,获得写入权限。
	// setting_stat 状态时，正在写入。即队列为空。其他EnQueue操作不能更改，只能由EnQueue操作更改为seted_stat,
	// 这两个状态在EnQueue操作时都表示为队列满。
	// seted_stat 状态时，已经写入,可以取出。DeQueue操作通过cas改变为getting_stat,获得取出权限。
	// getting_stat 状态时，正在取出。其他DeQueue操作不能更改，并且只能由DeQueue操作更改为geted_stat。
	data []stateNode
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
	q.deID = q.enID
	q.mod = modUint32(q.cap)
	q.cap = q.mod + 1
	q.len = 0
	q.data = make([]stateNode, q.cap)
}

// Init初始化长度为: DefauleSize.
func (q *LRQueue) Init() {
	q.InitWith()
}

// InitWith初始化长度为cap的queue,
// 如果未提供，则使用默认值: DefauleSize.
func (q *LRQueue) InitWith(cap ...int) {
	if len(cap) > 0 && cap[0] > 0 {
		q.cap = uint32(cap[0])
	}
	q.init()
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

// 数量
func (q *LRQueue) Size() int {
	return int(q.len)
}

// 根据enID,deID获取进队，出队对应的slot
func (q *LRQueue) getSlot(id uint32) *stateNode {
	return &q.data[int(id&q.mod)]
}

// EnQueue入队，如果队列满了，返回false。
func (q *LRQueue) EnQueue(val interface{}) bool {
	if q.Full() {
		return false
	}
	q.onceInit()
	var slot *stateNode
	// 获取 slot
	for {
		// 获取最新 enID,
		enID := atomic.LoadUint32(&q.enID)
		slot = q.getSlot(enID)
		stat := atomic.LoadUint32(&slot.state)

		// 检测state,seted_stat或者getting_stat表示队列满了。
		if stat == seted_stat || stat == getting_stat {
			// TODO 是否需要写入缓冲区,或者扩容
			// queue full,
			return false
		}
		// 获取slot写入权限,将state变为:had_Set_PushStat
		if casUint32(&slot.state, geted_stat, setting_stat) {
			// 成功获得slot
			// 先更新enID,让其他EnQueue更快执行
			atomic.AddUint32(&q.enID, 1)
			break
		}
	}
	// 向slot写入数据
	slot.store(val)
	atomic.AddUint32(&q.len, 1)

	// 更新state为seted_stat.
	atomic.StoreUint32(&slot.state, seted_stat)
	return true
}

// DeQueue出队，如果队列空了，返回false。
func (q *LRQueue) DeQueue() (val interface{}, ok bool) {
	if q.Empty() {
		return
	}
	q.onceInit()
	var slot *stateNode
	// 获取 slot
	for {
		// 获取最新 DeQueuePID,
		deID := atomic.LoadUint32(&q.deID)
		slot = q.getSlot(deID)
		stat := atomic.LoadUint32(&slot.state)

		// 检测 EnQueueStat 是否 can_set, 队列空了。
		if stat == geted_stat || stat == setting_stat {
			// queue empty,
			return nil, false
		}
		// 先读取值，再尝试删除旧slot,z 如果先删除旧slot,会被其他DeQueue清空slot
		val = slot.load()
		// 获取slot取出权限,将state变为getting_stat
		if casUint32(&slot.state, seted_stat, getting_stat) {
			// 成功取出slot
			atomic.AddUint32(&q.deID, 1)
			break
		}
	}
	// 释放slot
	atomic.AddUint32(&q.len, negativeOne)
	slot.free()

	// 更新EnQueueStat为geted_stat.因为	slot.free()已经清空了状态，这步可以不用写。
	atomic.StoreUint32(&slot.state, geted_stat)

	return val, true
}

var (
	errTimeOut = errors.New("超时")
)

// 带超时EnQueue入队。
func (q *LRQueue) PutWait(i interface{}, timeout time.Duration) (bool, error) {
	t := time.NewTicker(timeout)
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
