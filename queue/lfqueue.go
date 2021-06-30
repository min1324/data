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
	// 能在队列声明后，调用push或者pop操作前，初始化队列。
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
	// stat只有pushed和poped状态。
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
// 使用push或者pop前调用一次。
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
		// 因为置空队列，然后len=0两个操作期间，其他线程可能push。
		// I为Init，P为Push，运行路线可能：I将head指向tail,P通过push成功加入数据，len增加1，I将len置0.与预期1不相符。
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
			// 这里可能有并发push,增加len；即使有pop,也能保持newLen>=oldLen
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
			// 完成添加，将slot设置为可用，让等待的pop可以取走
			slot.change(pushed_stat)
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

		// Q->head 其他pop成功获得slot.指针已移动，重新取 head指针
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
		if slot.status() != pushed_stat {
			// TODO
			// push还没添加完成。
			// 方案1：直接返回nil。
			return nil, false

			// 方案2：等待push添加完成
			// continue
		}

		// 记录值，再尝试获取slot.
		// 如果先获取slot,再记录值。会出现的问题：
		// pop1通过cas获取slot后暂停。pop2获取到slot.next,调用headNode.free()，将slot释放掉
		// 此时slot已经被清空，获取到nil值。与预期不符。
		val := slot.load()
		if cas(&q.head, head, headNext) {
			// 成功取出slot,此时的slot可能被其他pop释放。
			// len-1
			atomic.AddUint32(&q.len, negativeOne)

			// 释放head指向上次pop操作的slot.
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

	len    uint32 // 队列当前数据长度
	cap    uint32 // 队列容量，自动向上调整至2^n
	mod    uint32 // cap-1,即2^n-1,用作取slot: data[ID&mod]
	popID  uint32 // 指向取出数据的位置:popID&mod
	pushID uint32 // 指向下次写入数据的位置:pushID&mod

	// 环形队列，大小必须是2的倍数。
	// state有4个状态。
	// 这两个状态在pop操作时都表示为队列空。
	// poped_stat 状态时，已经取出，可以写入。push操作通过cas改变为pushing_stat,获得写入权限。
	// pushing_stat 状态时，正在写入。即队列为空。其他push操作不能更改，只能由push操作更改为pushed_stat,
	// 这两个状态在push操作时都表示为队列满。
	// pushed_stat 状态时，已经写入,可以取出。pop操作通过cas改变为poping_stat,获得取出权限。
	// poping_stat 状态时，正在取出。其他pop操作不能更改，并且只能由pop操作更改为poped_stat。
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
	q.popID = q.pushID
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
	return q.pushID^q.cap == q.popID
	// return atomic.LoadUintptr(&q.len) == atomic.LoadUintptr(&q.cap)
	// return (q.popID&q.mod + 1) == (q.pushID & q.mod)
}

// 队列是否空
func (q *LRQueue) Empty() bool {
	// return atomic.LoadUintptr(&q.len) == 0
	return q.popID == q.pushID
}

// 数量
func (q *LRQueue) Size() int {
	return int(q.len)
}

// 根据pushID,popID获取进队，出队对应的slot
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
		// 获取最新 pushID,
		pushID := atomic.LoadUint32(&q.pushID)
		slot = q.getSlot(pushID)
		stat := atomic.LoadUint32(&slot.state)

		// 检测state,pushed_stat或者poping_stat表示队列满了。
		if stat == pushed_stat || stat == poping_stat {
			// TODO 是否需要写入缓冲区,或者扩容
			// queue full,
			return false
		}
		// 获取slot写入权限,将state变为:had_Set_PushStat
		if casUint32(&slot.state, poped_stat, pushing_stat) {
			// 成功获得slot
			// 先更新pushID,让其他push更快执行
			atomic.AddUint32(&q.pushID, 1)
			break
		}
	}
	// 向slot写入数据
	slot.store(val)
	atomic.AddUint32(&q.len, 1)

	// 更新state为pushed_stat.
	atomic.StoreUint32(&slot.state, pushed_stat)
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
		// 获取最新 popPID,
		popPID := atomic.LoadUint32(&q.popID)
		slot = q.getSlot(popPID)
		stat := atomic.LoadUint32(&slot.state)

		// 检测 pushStat 是否 can_set, 队列空了。
		if stat == poped_stat || stat == pushing_stat {
			// queue empty,
			return nil, false
		}

		// 获取slot取出权限,将state变为poping_stat
		if casUint32(&slot.state, pushed_stat, poping_stat) {
			// 成功取出slot
			// 先更新popPID,让其他pop更快执行
			atomic.AddUint32(&q.popID, 1)
			break
		}
	}
	// 读取value
	val = slot.load()
	atomic.AddUint32(&q.len, negativeOne)

	// 更新pushStat为poped_stat.
	atomic.StoreUint32(&slot.state, poped_stat)
	return val, true
}

var (
	errTimeOut = errors.New("超时")
)

// 带超时push入队。
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
