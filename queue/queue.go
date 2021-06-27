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
		q.Init()
	})
}

// Init initialize queue in lock.
func (q *Queue) Init() {
	q.head = unsafe.Pointer(&node{})
	q.tail = q.head
	q.count = 0
}

// Size queue element's number
func (q *Queue) Size() int {
	return int(atomic.LoadUintptr(&q.count))
}

// Enqueue puts the given value at the tail of the queue.
func (q *Queue) Push(i interface{}) {
	q.onceInit()
	slot := newNode(i)
	for {
		//先取一下尾指针和尾指针的next
		tail := atomic.LoadPointer(&q.tail)
		tailNode := (*node)(tail)
		next := tailNode.next

		//如果尾指针已经被移动了，则重新开始
		if tail != atomic.LoadPointer(&q.tail) {
			continue
		}

		//如果尾指针的 next 不为NULL，则 fetch 全局尾指针到next
		if next != nil {
			cas(&q.tail, tail, next)
			continue
		}

		//如果加入结点成功，则退出
		if cas(&tailNode.next, next, unsafe.Pointer(slot)) {
			atomic.AddUintptr(&q.count, 1)
			// try to swing tail to the inserted node
			cas(&q.tail, tail, unsafe.Pointer(slot))
			break
		}
	}
}

// Dequeue removes and returns the value at the head of the queue.
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

		// 如果是空队列
		if head == tail && next == nil {
			return nil
		}

		//如果 tail 指针落后了
		if head == tail && next != nil {
			cas(&q.tail, tail, next)
			continue
		}

		// head != tail
		if cas(&q.head, head, next) {
			nextNode := (*node)(next)
			atomic.AddUintptr(&q.count, ^uintptr(0))
			return nextNode.p
		}
	}
}

func loadNode(i *unsafe.Pointer) *node {
	return (*node)(atomic.LoadPointer(i))
}

func cas(p *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return atomic.CompareAndSwapPointer(
		p, unsafe.Pointer(old), unsafe.Pointer(new))
}
