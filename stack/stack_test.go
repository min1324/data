package stack_test

import (
	"data/stack"
	"sync"
	"sync/atomic"
	"testing"
)

type testFunc func()

func TestInit(t *testing.T) {
	var s stack.Stack
	t.Run("init", func(t *testing.T) {
		if s.Size() != 0 {
			t.Fatalf("init size != 0 :%d", s.Size())
		}
		if s.Pop() != nil {
			t.Fatalf("init Pop != nil :%v", s.Pop())
		}
		p := 1
		s.Push(p)
		v := s.Pop()
		if v.(int) != p {
			t.Fatalf("init push want:%d, real:%v", p, v)
		}
		s.Init()
		if s.Size() != 0 {
			t.Fatalf("init after Init err,size!=0,%d", s.Size())
		}
		if q := s.Pop(); q != nil {
			t.Fatalf("init after Init err,Pop!=nil,%v", q)
		}
	})
}

func TestConcurrentPush(t *testing.T) {
	var s stack.Stack
	var wg sync.WaitGroup

	n := 100
	m := 100

	for i := 0; i < m; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < n; i++ {
				s.Push(i)
			}
		}()
	}
	wg.Wait()
	if s.Size() != m*n {
		t.Fatalf("TestConcurrentPush err,push:%d,real:%d", n*m, s.Size())
	}
}

func TestConcurrentPop(t *testing.T) {
	var s stack.Stack
	var wg sync.WaitGroup

	n := 100
	m := 100
	var sum int64
	for i := 0; i < m*n; i++ {
		s.Push(i)
	}

	for i := 0; i < m; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s.Size() > 0 {
				t := s.Pop()
				if t != nil {
					atomic.AddInt64(&sum, 1)
				}
			}
		}()
	}
	wg.Wait()

	if sum != int64(m*n) {
		t.Fatalf("TestConcurrentPush err,push:%d,pop:%d", n*m, sum)
	}
}

func TestConcurrentPushPop(t *testing.T) {
	// push routine push total sumPush item into it.
	// pop routine pop until recive push's finish signal
	// finally check if s.Size()+sumPop == sumPush
	var s stack.Stack
	var popWG sync.WaitGroup
	var pushWG sync.WaitGroup

	n := 1000
	m := 100
	exit := make(chan struct{}, m)

	var sumPush, sumPop int64
	for i := 0; i < m; i++ {
		pushWG.Add(1)
		go func() {
			defer pushWG.Done()
			for j := 0; j < n; j++ {
				s.Push(j)
				atomic.AddInt64(&sumPush, 1)
			}
		}()
		popWG.Add(1)
		go func() {
			defer popWG.Done()
			for {
				select {
				case <-exit:
					return
				default:
					t := s.Pop()
					if t != nil {
						atomic.AddInt64(&sumPop, 1)
					}
				}
			}
		}()
	}
	pushWG.Wait()
	close(exit)
	popWG.Wait()
	exit = nil

	if sumPop+int64(s.Size()) != sumPush {
		t.Fatalf("TestConcurrentPushPop err,Push:%d,pop:%d,instack:%d", sumPush, sumPop, s.Size())
	}
}
