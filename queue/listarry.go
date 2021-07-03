package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// 数组加链表队列
// 链表队列用于缓冲
type lAQueue struct {
}

// 单生产者，多消费者队列
type SPMCQueue struct {
	vals     []eface
	headtail uint64
	once     sync.Once
}

type eface struct {
	typ, val unsafe.Pointer
}

const queueBits = 32
const queueLimit = (1 << queueBits) / 4

func (s *SPMCQueue) unpack(ptrs uint64) (head, tail uint32) {
	const mask = 1<<queueBits - 1
	head = uint32((ptrs >> queueBits) & mask)
	tail = uint32(ptrs & mask)
	return
}

func (s *SPMCQueue) pack(head, tail uint32) (ptrs uint64) {
	const mask = 1<<queueBits - 1
	return (uint64(head)<<queueBits | uint64(tail&mask))
}

func (s *SPMCQueue) pushHead(val interface{}) bool {
	ptrs := atomic.LoadUint64(&s.headtail)
	head, tail := s.unpack(ptrs)
	if (tail+uint32(len(s.vals)))&(1<<queueBits-1) == head {
		// queue full
		return false
	}
	slot := &s.vals[head&uint32((len(s.vals)-1))]
	// Check if the head slot has been released by popTail.
	typ := atomic.LoadPointer(&slot.typ)
	if typ != nil {
		// Another goroutine is still cleaning up the tail, so
		// the queue is actually still full.
		return false
	}
	// typ == nil
	// The head slot is free, so we own it.
	if val == nil {
		val = queueNil(nil)
	}
	*(*interface{})(unsafe.Pointer(slot)) = val
	// Increment head. This passes ownership of slot to popTail
	// and acts as a store barrier for writing the slot.
	atomic.AddUint64(&s.headtail, 1<<queueBits)
	return true
}

func (s *SPMCQueue) popHead() (interface{}, bool) {
	var slot *eface
	for {
		ptrs := atomic.LoadUint64(&s.headtail)
		head, tail := s.unpack(ptrs)
		if head == tail {
			return nil, false
		}
		head--
		ptrs2 := s.pack(head, tail)
		if atomic.CompareAndSwapUint64(&s.headtail, ptrs, ptrs2) {
			// We successfully took back slot.
			slot = &s.vals[head&uint32(len(s.vals)-1)]
			break
		}
	}
	val := *(*interface{})(unsafe.Pointer(slot))
	if val == queueNil(nil) {
		val = nil
	}
	// Zero the slot. Unlike popTail, this isn't racing with
	// pushHead, so we don't need to be careful here.
	*slot = eface{}
	return val, true
}

func (s *SPMCQueue) popTail() (interface{}, bool) {
	var slot *eface
	for {
		ptrs := atomic.LoadUint64(&s.headtail)
		head, tail := s.unpack(ptrs)
		if head == tail {
			return nil, false
		}

		ptrs2 := s.pack(head, tail+1)
		if atomic.CompareAndSwapUint64(&s.headtail, ptrs, ptrs2) {
			// We successfully took back slot.
			slot = &s.vals[tail&uint32(len(s.vals)-1)]
			break
		}
	}
	val := *(*interface{})(unsafe.Pointer(slot))
	if val == queueNil(nil) {
		val = nil
	}
	// Tell pushHead that we're done with this slot. Zeroing the
	// slot is also important so we don't leave behind references
	// that could keep this object live longer than necessary.
	//
	// We write to val first and then publish that we're done with
	// this slot by atomically writing to typ.
	slot.val = nil
	atomic.StorePointer(&slot.typ, nil)

	return val, true
}
