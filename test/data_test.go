package data_test

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/min1324/data/queue"
)

type test struct {
	setup func(*testing.T, SQInterface)
	perG  func(*testing.T, SQInterface)
}

type testFunc func(*testing.T, SQInterface)

func testStack(t *testing.T, test test) {
	for _, m := range [...]SQInterface{
		&UnsafeQueue{},
		&queue.DLQueue{},
		&queue.DRQueue{},
		&queue.LLQueue{},
		&queue.SAQueue{},
		&queue.SLQueue{},
		&queue.SRQueue{},
		&queue.Slice{},

		// &MutexStack{},
		// &stack.Stack{},
	} {
		t.Run(fmt.Sprintf("%T", m), func(t *testing.T) {
			m = reflect.New(reflect.TypeOf(m).Elem()).Interface().(SQInterface)
			m.Init()
			if q, ok := m.(*queue.LRQueue); ok {
				q.InitWith(queueMaxSize)
			}
			if q, ok := m.(*queue.DRQueue); ok {
				q.InitWith(queueMaxSize)
			}
			if q, ok := m.(*queue.SRQueue); ok {
				q.InitWith(queueMaxSize)
			}

			if test.setup != nil {
				test.setup(t, m)
			}
			test.perG(t, m)
		})
	}
}

func TestInit(t *testing.T) {

	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
		},
		perG: func(t *testing.T, s SQInterface) {
			// 初始化测试，
			if s.Size() != 0 {
				t.Fatalf("init size != 0 :%d", s.Size())
			}

			if v, ok := s.DeQueue(); ok {
				t.Fatalf("init DeQueue != nil :%v", v)
			}
			s.Init()
			if s.Size() != 0 {
				t.Fatalf("Init err,size!=0,%d", s.Size())
			}

			if v, ok := s.DeQueue(); ok {
				t.Fatalf("Init DeQueue != nil :%v", v)
			}
			s.EnQueue("a")
			s.EnQueue("a")
			s.EnQueue("a")
			s.Init()
			if s.Size() != 0 {
				t.Fatalf("after EnQueue Init err,size!=0,%d", s.Size())
			}
			if v, ok := s.DeQueue(); ok {
				t.Fatalf("after EnQueue Init DeQueue != nil :%v", v)
			}

			s.Init()
			// EnQueue,DeQueue测试
			p := 1
			s.EnQueue(p)
			if s.Size() != 1 {
				t.Fatalf("after EnQueue err,size!=1,%d", s.Size())
			}

			if v, ok := s.DeQueue(); !ok || v != p {
				t.Fatalf("EnQueue want:%d, real:%v", p, v)
			}

			// size 测试
			s.Init()
			var n = 10
			for i := 0; i < n; i++ {
				s.EnQueue(i)
			}
			if s.Size() != n {
				t.Fatalf("Size want:%d, real:%v", n, s.Size())
			}

			// 储存顺序测试,数组队列可能满
			// stack顺序反过来
			s.Init()
			array := [...]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
			for i := range array {
				s.EnQueue(i)
				array[i] = i // queue用这种
				// array[len(array)-i-1] = i  // stack用这种方式
			}
			for i := 0; i < len(array); i++ {
				v, ok := s.DeQueue()
				if !ok || v != array[i] {
					t.Fatalf("array want:%d, real:%v", array[i], v)
				}
			}

			s.Init()
			// 空值测试
			var nullPtrs = unsafe.Pointer(nil)
			s.EnQueue(nullPtrs)

			if v, ok := s.DeQueue(); !ok || nullPtrs != v {
				t.Fatalf("EnQueue nil want:%v, real:%v", nullPtrs, v)
			}
			var null = new(interface{})
			s.EnQueue(null)
			if v, ok := s.DeQueue(); !ok || null != v {
				t.Fatalf("EnQueue nil want:%v, real:%v", null, v)
			}
		},
	})
}

func TestEnQueue(t *testing.T) {
	const maxSize = 1 << 10

	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
		},
		perG: func(t *testing.T, s SQInterface) {
			for i := 0; i < maxSize; i++ {
				s.EnQueue(i)
			}

			if s.Size() != maxSize {
				t.Fatalf("TestConcurrentEnQueue err,EnQueue:%d,real:%d", maxSize, s.Size())
			}
		},
	})
}

func TestDeQueue(t *testing.T) {
	const maxSize = 1 << 10
	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
		},
		perG: func(t *testing.T, s SQInterface) {
			for i := 0; i < maxSize; i++ {
				s.EnQueue(i)
			}

			sum := 0
			for i := 0; i < maxSize; i++ {
				_, ok := s.DeQueue()
				if ok {
					sum += 1
				}
			}

			if s.Size()+sum != maxSize {
				t.Fatalf("TestDeQueue err,EnQueue:%d,DeQueue:%d,size:%d", maxSize, sum, s.Size())
			}
		},
	})
}

func TestConcurrentEnQueue(t *testing.T) {
	const maxGo, maxNum = 4, 1 << 8
	const maxSize = maxGo * maxNum

	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
			if _, ok := s.(*UnsafeQueue); ok {
				t.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(t *testing.T, s SQInterface) {
			var wg sync.WaitGroup
			for i := 0; i < maxGo; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < maxNum; i++ {
						s.EnQueue(i)
					}
				}()
			}
			wg.Wait()
			if s.Size() != maxSize {
				t.Fatalf("TestConcurrentEnQueue err,EnQueue:%d,real:%d", maxSize, s.Size())
			}
		},
	})
}

func TestConcurrentDeQueue(t *testing.T) {
	const maxGo, maxNum = 4, 1 << 10
	const maxSize = maxGo * maxNum

	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
			if _, ok := s.(*UnsafeQueue); ok {
				t.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(t *testing.T, s SQInterface) {
			var wg sync.WaitGroup
			var sum int64
			var EnQueueSum int64
			for i := 0; i < maxSize; i++ {
				s.EnQueue(i)
				EnQueueSum += 1
			}

			for i := 0; i < maxGo; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for s.Size() > 0 {
						_, ok := s.DeQueue()
						if ok {
							atomic.AddInt64(&sum, 1)
						}
					}
				}()
			}
			wg.Wait()

			if sum+int64(s.Size()) != int64(EnQueueSum) {
				t.Fatalf("TestConcurrentEnQueue err,EnQueue:%d,DeQueue:%d,size:%d", EnQueueSum, sum, s.Size())
			}
		},
	})
}

func TestConcurrentEnQueueDeQueue(t *testing.T) {
	const maxGo, maxNum = 4, 1 << 10
	const maxSize = maxGo * maxNum

	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
			if _, ok := s.(*UnsafeQueue); ok {
				t.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(t *testing.T, s SQInterface) {
			// EnQueue routine EnQueue total sumEnQueue item into it.
			// DeQueue routine DeQueue until recive EnQueue's finish signal
			// finally check if s.Size()+sumDeQueue == sumEnQueue
			var DeQueueWG sync.WaitGroup
			var EnQueueWG sync.WaitGroup

			exit := make(chan struct{}, maxGo)

			var sumEnQueue, sumDeQueue int64
			for i := 0; i < maxGo; i++ {
				EnQueueWG.Add(1)
				go func() {
					defer EnQueueWG.Done()
					for j := 0; j < maxNum; j++ {
						s.EnQueue(j)
						atomic.AddInt64(&sumEnQueue, 1)
					}
				}()
				DeQueueWG.Add(1)
				go func() {
					defer DeQueueWG.Done()
					for {
						select {
						case <-exit:
							return
						default:
							_, ok := s.DeQueue()
							if ok {
								atomic.AddInt64(&sumDeQueue, 1)
							}
						}
					}
				}()
			}
			EnQueueWG.Wait()
			close(exit)
			DeQueueWG.Wait()
			exit = nil

			if sumDeQueue+int64(s.Size()) != sumEnQueue {
				t.Fatalf("TestConcurrentEnQueueDeQueue err,EnQueue:%d,DeQueue:%d,instack:%d", sumEnQueue, sumDeQueue, s.Size())
			}
		},
	})
}
