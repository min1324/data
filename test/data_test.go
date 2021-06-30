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
		// &UnsafeQueue{},
		// &queue.DLQueue{},
		// &queue.DRQueue{},
		// &queue.LAQueue{},
		// &queue.LFQueue{},
		// &queue.SAQueue{},
		// &queue.SLQueue{},
		// &queue.SRQueue{},
		// &queue.Slice{},

		// &MutexStack{},
		// &stack.Stack{},
	} {
		t.Run(fmt.Sprintf("%T", m), func(t *testing.T) {
			m = reflect.New(reflect.TypeOf(m).Elem()).Interface().(SQInterface)
			m.Init()
			if q, ok := m.(*queue.LAQueue); ok {
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
			s.Push("a")
			s.Push("a")
			s.Push("a")
			s.Init()
			if s.Size() != 0 {
				t.Fatalf("after push Init err,size!=0,%d", s.Size())
			}
			if s.Pop() != nil {
				t.Fatalf("after push Init Pop != nil :%v", s.Pop())
			}

			s.Init()
			// push,pop测试
			p := 1
			s.Push(p)
			if s.Size() != 1 {
				t.Fatalf("after push err,size!=1,%d", s.Size())
			}
			v := s.Pop()
			if v != p {
				t.Fatalf("push want:%d, real:%v", p, v)
			}

			// size 测试
			s.Init()
			var n = 10
			for i := 0; i < n; i++ {
				s.Push(i)
			}
			if s.Size() != n {
				t.Fatalf("Size want:%d, real:%v", n, s.Size())
			}

			// 储存顺序测试,数组队列可能满
			// stack顺序反过来
			s.Init()
			array := [...]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
			for i := range array {
				s.Push(i)
				array[i] = i // queue用这种
				// array[len(array)-i-1] = i  // stack用这种方式
			}
			for i := 0; i < len(array); i++ {
				v := s.Pop()
				if v != array[i] {
					t.Fatalf("array want:%d, real:%v", array[i], v)
				}
			}

			s.Init()
			// 空值测试
			var nullPtrs = unsafe.Pointer(nil)
			s.Push(nullPtrs)
			np := s.Pop()
			if np != nullPtrs {
				t.Fatalf("push nil want:%v, real:%v", nullPtrs, np)
			}
			var null = new(interface{})
			s.Push(null)
			nv := s.Pop()
			if nv != null {
				t.Fatalf("push nil want:%v, real:%v", null, nv)
			}
		},
	})
}

func TestPush(t *testing.T) {
	const maxSize = 1 << 10

	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
		},
		perG: func(t *testing.T, s SQInterface) {
			for i := 0; i < maxSize; i++ {
				s.Push(i)
			}

			if s.Size() != maxSize {
				t.Fatalf("TestConcurrentPush err,push:%d,real:%d", maxSize, s.Size())
			}
		},
	})
}

func TestPop(t *testing.T) {
	const maxSize = 1 << 10
	testStack(t, test{
		setup: func(t *testing.T, s SQInterface) {
		},
		perG: func(t *testing.T, s SQInterface) {
			for i := 0; i < maxSize; i++ {
				s.Push(i)
			}

			sum := 0
			for i := 0; i < maxSize; i++ {
				t := s.Pop()
				if t != nil {
					sum += 1
				}
			}

			if s.Size()+sum != maxSize {
				t.Fatalf("TestPop err,push:%d,pop:%d,size:%d", maxSize, sum, s.Size())
			}
		},
	})
}

func TestConcurrentPush(t *testing.T) {
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
						s.Push(i)
					}
				}()
			}
			wg.Wait()
			if s.Size() != maxSize {
				t.Fatalf("TestConcurrentPush err,push:%d,real:%d", maxSize, s.Size())
			}
		},
	})
}

func TestConcurrentPop(t *testing.T) {
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
			var pushSum int64
			for i := 0; i < maxSize; i++ {
				s.Push(i)
				pushSum += 1
			}

			for i := 0; i < maxGo; i++ {
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

			if sum+int64(s.Size()) != int64(pushSum) {
				t.Fatalf("TestConcurrentPush err,push:%d,pop:%d,size:%d", pushSum, sum, s.Size())
			}
		},
	})
}

func TestConcurrentPushPop(t *testing.T) {
	const maxGo, maxNum = 4, 1 << 10
	const maxSize = maxGo * maxNum

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

			exit := make(chan struct{}, maxGo)

			var sumPush, sumPop int64
			for i := 0; i < maxGo; i++ {
				pushWG.Add(1)
				go func() {
					defer pushWG.Done()
					for j := 0; j < maxNum; j++ {
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
