package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	queueBits  = 32
	queueLimit = (1 << queueBits) >> 2
)

// poolChain is a dynamically-sized version of LRQueue.
type Chain struct {
	once sync.Once
	// tail is the LRQueue to push to. This is only accessed
	// by the producers, so reads and writes must be atomic.
	tail *chainElt

	// head is the LRQueue to pop from. This is accessed
	// by consumers, so reads and writes must be atomic.
	head *chainElt
}

type chainElt struct {
	// DRQueue
	LRQueue

	// next and prev link to the adjacent poolChainElts in this
	// poolChain.
	//
	// next is written atomically by the producer and read
	// atomically by the consumer. It only transitions from nil to
	// non-nil.
	//
	// prev is written atomically by the consumer and read
	// atomically by the producer. It only transitions from
	// non-nil to nil.
	next *chainElt
}

func storeChainElt(pp **chainElt, v *chainElt) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(pp)), unsafe.Pointer(v))
}

func loadChainElt(pp **chainElt) *chainElt {
	return (*chainElt)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(pp))))
}

func casChainElt(pp **chainElt, old, new *chainElt) bool {
	return atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(pp)), unsafe.Pointer(old), unsafe.Pointer(new))
}

func (c *Chain) onceInit() {
	c.once.Do(func() {
		c.init()
	})
}

func (c *Chain) init() {
	newNode := &chainElt{next: nil}
	newCap := 8
	newNode.InitWith(newCap)
	storeChainElt(&c.head, newNode)
	storeChainElt(&c.tail, newNode)
}

func (c *Chain) Push(val interface{}) bool {
	c.onceInit()
	for {
		tail := loadChainElt(&c.tail)
		// 成功加入队列，直接返回
		if tail.EnQueue(val) {
			return true
		}
		// 队列不存在或满了，需要扩容。
		newNode := &chainElt{}
		newCap := tail.Cap() << 1
		if newCap >= queueLimit {
			newCap = queueLimit
		}
		newNode.InitWith(newCap)
		// 将newTail加入chain
		if casChainElt(&c.tail, tail, newNode) {
			storeChainElt(&tail.next, newNode)
			// 优先权
			if newNode.EnQueue(val) {
				return true
			}
		}
	}
}

func (c *Chain) Pop() (val interface{}, ok bool) {
	c.onceInit()
	head := loadChainElt(&c.head)
	if head == nil {
		return
	}
	for {
		// It's important that we load the next pointer
		// *before* popping. In general, head may be
		// transiently empty, but if next is non-nil before
		// the pop and the pop fails, then head is permanently
		// empty, which is the only condition under which it's
		// safe to drop head from the chain.
		head2 := loadChainElt(&head.next)
		if val, ok = head.DeQueue(); ok {
			return
		}
		// 当前的头部没有值，切换到下一个节点pop
		if head2 == nil {
			return nil, false
		}
		// The tail of the chain has been drained, so move on
		// to the next dequeue. Try to drop it from the chain
		// so the next pop doesn't have to look at the empty
		// dequeue again.
		if casChainElt(&c.head, head, head2) {
			// We won the race. Clear the prev pointer so
			// the garbage collector can collect the empty
			// dequeue and so popHead doesn't back up
			// further than necessary.
		}
		head = head2
	}
}

func (c *Chain) Init() {
	c.init()
	// for {
	// 	tail := loadChainElt(&c.tail)
	// 	if casChainElt(&c.tail, c.tail, nil) {
	// 		storeChainElt(&c.head, nil)
	// 		for tail != nil {
	// 			p := tail
	// 			p.Init()
	// 			p.next = nil
	// 		}
	// 		break
	// 	}
	// }
}

func (c *Chain) Size() int {
	head := loadChainElt(&c.head)
	var sum = 0
	for head != nil {
		sum += head.Size()
		head = head.next
	}
	return sum
}

func (c *Chain) EnQueue(val interface{}) bool { return c.Push(val) }
func (c *Chain) DeQueue() (interface{}, bool) { return c.Pop() }
