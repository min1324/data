package data_test

import (
	"data/queue"
	"data/stack"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
)

type test struct {
	setup func(*testing.T, SQInterface)
	perG  func(*testing.T, SQInterface)
}

type testFunc func(*testing.T, SQInterface)

func testStack(t *testing.T, test test) {
	for _, m := range [...]SQInterface{
		&UnsafeQueue{},
		&MutexQueue{},
		&queue.Queue{},
		&MutexSlice{},
		&queue.Slice{},
		&MutexStack{},
		&stack.Stack{},
	} {
		t.Run(fmt.Sprintf("%T", m), func(t *testing.T) {
			m = reflect.New(reflect.TypeOf(m).Elem()).Interface().(SQInterface)
			if test.setup != nil {
				test.setup(t, m)
			}
		})
	}
}

func TestInit(t *testing.T) {
	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
		},
		perG: func(t *testing.T, s SQInterface) {
			if s.Size() != 0 {
				t.Fatalf("init size != 0 :%d", s.Size())
			}
			if s.Pop() != nil {
				t.Fatalf("init Pop != nil :%v", s.Pop())
			}
			s.Init()
			if s.Size() != 0 {
				t.Fatalf("Init err,size!=0,%d", s.Size())
			}
			if s.Pop() != nil {
				t.Fatalf("Init Pop != nil :%v", s.Pop())
			}
			p := 1
			s.Push(p)
			v := s.Pop()
			if v != p {
				t.Fatalf("init push want:%d, real:%v", p, v)
			}
			var null = unsafe.Pointer(nil)
			s.Push(null)
			nv := s.Pop()
			if nv != null {
				t.Fatalf("push nil want:%v, real:%v", null, nv)
			}
			nullp := new(interface{})
			s.Push(nullp)
			np := s.Pop()
			if np != nullp {
				t.Fatalf("push interface want:%v, real:%v", nullp, np)
			}
		},
	})

}

func TestConcurrentPush(t *testing.T) {
	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
			if _, ok := s.(*UnsafeQueue); ok {
				t.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(t *testing.T, s SQInterface) {
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
		},
	})
}

func TestConcurrentPop(t *testing.T) {
	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
			if _, ok := s.(*UnsafeQueue); ok {
				t.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(t *testing.T, s SQInterface) {
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
		},
	})
}

func TestConcurrentPushPop(t *testing.T) {
	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
			if _, ok := s.(*UnsafeQueue); ok {
				t.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(t *testing.T, s SQInterface) {
			// push routine push total sumPush item into it.
			// pop routine pop until recive push's finish signal
			// finally check if s.Size()+sumPop == sumPush
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
		},
	})
}
