package stack_test

import "sync"

// MutexStack stack with mutex
type MutexStack struct {
	top   *node
	count int
	mu    sync.Mutex
}

type node struct {
	p    interface{}
	next *node
}

func newNode(i interface{}) *node {
	return &node{p: i}
}

func (s *MutexStack) Push(i interface{}) {
	s.mu.Lock()
	n := newNode(i)
	n.next = s.top
	s.top = n
	s.mu.Unlock()
}

func (s *MutexStack) Pop() interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.top == nil {
		return nil
	}
	top := s.top
	s.top = top.next
	return top.p
}
