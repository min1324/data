package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// LFQueue is a lock-free unbounded linked list queue.
type LFQueue struct {
	// 声明队列后，如果没有调用Init(),队列不能使用
	// 为解决这个问题，加入一次性操作。
	// 能在队列声明后，调用push或者pop操作前，初始化队列。
	// 见 onceInit()
	once sync.Once

	// len is num of value store in queue
	len uintptr

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
	head unsafe.Pointer
	tail unsafe.Pointer
}

type node struct {
	// TODO 用unsafe.Pointer？
	p interface{}

	// typ标志node是否可用
	// 为空时，说明其他push还没完成添加操作，需要等待添加完成才可以取出
	// 非空时，说明已经添加完成，可以取出了。
	typ  unsafe.Pointer
	next unsafe.Pointer
}

var empty = unsafe.Pointer(new(interface{}))

// New return an empty lock-free unbound list Queue
func New() *LFQueue {
	var q LFQueue
	q.onceInit()
	return &q
}

// 一次性初始化,线程安全。
func (q *LFQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

// 线程不安全初始化，只在声明队列后，
// 使用push或者pop前调用一次。
func (q *LFQueue) init() {
	q.head = unsafe.Pointer(&node{})
	q.tail = q.head
	q.len = 0
}

// Init initialize queue
func (q *LFQueue) Init() {
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
		// 两种方案：释放旧node时，一个一个的减少len,或在一次性更新len
		// 期间调用Size()无法获得正确的长度。

		// 方案1,一次性更新len
		oldLen := atomic.LoadUintptr(&q.len)
		if cas(&q.head, head, tail) {
			for {
				newLen := atomic.LoadUintptr(&q.len)
				detla := newLen - oldLen
				if atomic.CompareAndSwapUintptr(&q.len, newLen, detla) {
					break
				}
			}

			for head != tail && head != nil {
				node := (*node)(head)
				head = node.next
				node.free()
			}
			return
		}

		// // 方案2,释放旧node,len-1
		// if cas(&q.head, head, tail) {
		// 	for head != tail && head != nil {
		// 		node := (*node)(head)
		// 		head = node.next
		// 		node.free()
		// 		atomic.AddUintptr(&q.len, null)
		// 	}
		// 	return
		// }
	}
}

// Size queue element's number
func (q *LFQueue) Size() int {
	return int(atomic.LoadUintptr(&q.len))
}

func newNode(i interface{}) *node {
	return &node{p: i}
	// return &node{p: unsafe.Pointer(&i)}
}

// Push puts the given value at the tail of the queue.
func (q *LFQueue) Push(i interface{}) {
	q.onceInit()
	slot := newNode(i)
	slotPtr := unsafe.Pointer(slot)
	for {
		// 先取一下尾指针和尾指针的next
		tail := atomic.LoadPointer(&q.tail)
		tailNode := (*node)(tail)
		next := tailNode.next

		// 如果尾指针已经被移动了，则重新开始
		if tail != atomic.LoadPointer(&q.tail) {
			continue
		}

		// 如果尾指针的next!=nil，则提升tail直到指向最后一个位置
		if next != nil {
			cas(&q.tail, tail, next)
			continue
		}

		// next==nil,确定是最后一个
		if cas(&tailNode.next, next, slotPtr) {
			// 已经成功加入节点，尝试将 tail 提升到最新。
			cas(&q.tail, tail, slotPtr)
			// 更新len
			atomic.AddUintptr(&q.len, 1)
			// 完成添加，将slot设置为可用，让等待的pop可以取走
			slot.changeStatCanUse()
			break
		}
	}
}

func (n *node) changeStatCanUse() {
	atomic.StorePointer(&n.typ, empty)
}

// Pop removes and returns the value at the head of the queue.
// It returns nil if the queue is empty.
func (q *LFQueue) Pop() interface{} {
	q.onceInit()
	for {
		//取出头指针，尾指针，和第一个node指针
		head := atomic.LoadPointer(&q.head)
		tail := atomic.LoadPointer(&q.tail)
		headNode := (*node)(head)
		headNext := headNode.next

		// Q->head 其他pop成功获得slot.指针已移动，重新取 head指针
		if head != atomic.LoadPointer(&q.head) {
			continue
		}

		if head == tail {
			if headNext == nil {
				// 空队列返回
				return nil
			}
			// 此时head,tail指向同一个位置，并且有新增node
			// tail指针落后了,需要提升tail到下一个node,(tailNext==headNext)
			cas(&q.tail, tail, headNext)
		} else {
			slot := (*node)(headNext)
			if slot.notCanUse() {
				// TODO
				// push还没添加完成。

				// 方案1：直接返回nil。
				return nil

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
				atomic.AddUintptr(&q.len, ^uintptr(0))

				// 释放旧head指向slot.
				headNode.free()

				return val
			}
		}
	}
}

func (n *node) notCanUse() bool {
	return atomic.LoadPointer(&n.typ) == nil
}

func (n *node) load() interface{} {
	return n.p
	//return *(*interface{})(n.p)
}

func (n *node) free() {
	n.next = nil
	n.p = nil
	n.typ = nil
}

func cas(p *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return atomic.CompareAndSwapPointer(p, old, new)
}
func (q *LFQueue) Range(f func(interface{})) {
	head := q.head
	tail := q.tail
	if head == tail {
		return
	}

	for head != tail {
		headNode := (*node)(head)
		n := (*node)(headNode.next)
		f(n.load())
		head = headNode.next
	}
}
