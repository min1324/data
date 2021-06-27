package queue_test

import (
	"data/queue"
	"sync"
	"sync/atomic"
	"testing"
)

type testFunc func()

func TestInit(t *testing.T) {
	var q queue.Queue
	t.Run("init", func(t *testing.T) {
		if q.Size() != 0 {
			t.Fatalf("init size != 0 :%d", q.Size())
		}
		if q.Pop() != nil {
			t.Fatalf("init Pop != nil :%v", q.Pop())
		}
		p := 1
		q.Push(p)
		v := q.Pop()
		if v.(int) != p {
			t.Fatalf("init push want:%d, real:%v", p, v)
		}
		q.Init()
		if q.Size() != 0 {
			t.Fatalf("init after Init err,size!=0,%d", q.Size())
		}
		if q := q.Pop(); q != nil {
			t.Fatalf("init after Init err,Pop!=nil,%v", q)
		}
	})
}

func TestConcurrentPush(t *testing.T) {
	var q queue.Queue
	var wg sync.WaitGroup

	n := 100
	m := 100

	for i := 0; i < m; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < n; i++ {
				q.Push(i)
			}
		}()
	}
	wg.Wait()
	if q.Size() != m*n {
		t.Fatalf("TestConcurrentPush err,push:%d,real:%d", n*m, q.Size())
	}
}

func TestConcurrentPop(t *testing.T) {
	var q queue.Queue
	var wg sync.WaitGroup

	n := 100
	m := 100
	var sum int64
	for i := 0; i < m*n; i++ {
		q.Push(i)
	}

	for i := 0; i < m; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for q.Size() > 0 {
				t := q.Pop()
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
	// pop routine pop until recive push'q finish signal
	// finally check if q.Size()+sumPop == sumPush
	var q queue.Queue
	var popWG sync.WaitGroup
	var pushWG sync.WaitGroup

	n := 10000
	m := 100
	exit := make(chan struct{}, m)

	var sumPush, sumPop int64
	for i := 0; i < m; i++ {
		pushWG.Add(1)
		go func() {
			defer pushWG.Done()
			for j := 0; j < n; j++ {
				q.Push(j)
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
					t := q.Pop()
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

	if sumPop+int64(q.Size()) != sumPush {
		t.Fatalf("TestConcurrentPushPop err,Push:%d,pop:%d,instack:%d", sumPush, sumPop, q.Size())
	}
}
