package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// Queue a lock-free concurrent FIFO queues.
type Queue struct {
	head  unsafe.Pointer // head not store value,the frist value in head.next if exists.
	tail  unsafe.Pointer // tail.next if not nil, need promote.
	count uintptr        // count is num of value store in queue
	once  sync.Once      // use for init
}

type node struct {
	p    interface{}
	next unsafe.Pointer
}

func newNode(i interface{}) *node {
	return &node{p: i}
	// return &node{p: unsafe.Pointer(&i)}
}

func (n *node) load() interface{} {
	return n.p
	//return *(*interface{})(n.p)
}

// New return an empty Queue
func New() *Queue {
	var q Queue
	q.onceInit()
	return &q
}

// onceInit initialize queue
// if use var q Queue statement struct
// user may forget Init Queue,
// so use a once func in Push to init queue.
func (q *Queue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *Queue) init() {
	q.head = unsafe.Pointer(&node{})
	q.tail = q.head
	q.count = 0
}

// Init initialize queue
func (q *Queue) Init() {
	q.onceInit()
	for {
		s := q.head // start node
		e := q.tail // end node
		if s == e {
			return
		}
		if cas(&q.head, s, q.tail) { // head point to tail means queue empty
			// free queue [s ->...-> e]
			for s != e {
				node := (*node)(s)
				s = node.next
				node.next = nil
				atomic.AddUintptr(&q.count, null)
			}
			return
		}
	}
}

// Size queue element's number
func (q *Queue) Size() int {
	return int(atomic.LoadUintptr(&q.count))
}

// Push puts the given value at the tail of the queue.
func (q *Queue) Push(i interface{}) {
	q.onceInit()
	slot := unsafe.Pointer(newNode(i))
	for {
		// 先取一下尾指针和尾指针的next
		tail := atomic.LoadPointer(&q.tail)
		tailNode := (*node)(tail)
		next := tailNode.next

		// 如果尾指针已经被移动了，则重新开始
		if tail != atomic.LoadPointer(&q.tail) {
			continue
		}

		// 如果尾指针的 next 不为NULL，则 fetch 全局尾指针到next
		if next != nil {
			cas(&q.tail, tail, next)
			continue
		}

		// next==nil,确定是最后一个
		// 如果加入结点成功，则退出
		if cas(&tailNode.next, next, slot) {
			// 已经成功加入节点，尝试将 tail 提升到最新。
			cas(&q.tail, tail, slot)
			atomic.AddUintptr(&q.count, 1)
			break
		}
	}
}

// Pop removes and returns the value at the head of the queue.
// It returns nil if the queue is empty.
func (q *Queue) Pop() interface{} {
	q.onceInit()
	for {
		//取出头指针，尾指针，和第一个元素的指针
		head := atomic.LoadPointer(&q.head)
		tail := atomic.LoadPointer(&q.tail)
		headNode := (*node)(head)
		next := headNode.next

		// Q->head 指针已移动，重新取 head指针
		if head != atomic.LoadPointer(&q.head) {
			continue
		}

		// queue 正常
		if head == tail {
			// 空队列返回
			if next == nil {
				return nil
			}
			// tail 指针落后了
			cas(&q.tail, tail, next)
		} else {
			if cas(&q.head, head, next) {
				nextNode := (*node)(next)
				atomic.AddUintptr(&q.count, ^uintptr(0))
				headNode.next = nil // free old dummy node
				return nextNode.load()
			}
		}
	}
}

func loadNode(i *unsafe.Pointer) *node {
	return (*node)(atomic.LoadPointer(i))
}

func cas(p *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return atomic.CompareAndSwapPointer(p, old, new)
}
