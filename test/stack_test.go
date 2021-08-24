package data_test

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/min1324/data/stack"
)

type stackStruct struct {
	setup func(*testing.T, SInterface)
	perG  func(*testing.T, SInterface)
}

func stackMap(t *testing.T, test stackStruct) {
	for _, m := range [...]SInterface{
		&stack.LAStack{},
		&stack.LLStack{},
		&stack.SAStack{},
		&stack.SLStack{},
	} {
		t.Run(fmt.Sprintf("%T", m), func(t *testing.T) {
			m = reflect.New(reflect.TypeOf(m).Elem()).Interface().(SInterface)
			m.Init()
			t.Log("init:", m.Size())

			if test.setup != nil {
				test.setup(t, m)
			}
			test.perG(t, m)
		})
	}
}

func TestStackInit(t *testing.T) {

	stackMap(t, stackStruct{
		setup: func(t *testing.T, s SInterface) {
		},
		perG: func(t *testing.T, s SInterface) {
			// 初始化测试，
			if s.Size() != 0 {
				t.Fatalf("init size != 0 :%d", s.Size())
			}

			if v, ok := s.Pop(); ok {
				t.Fatalf("init Pop != nil :%v", v)
			}
			s.Init()
			if s.Size() != 0 {
				t.Fatalf("Init err,size!=0,%d", s.Size())
			}

			if v, ok := s.Pop(); ok {
				t.Fatalf("Init Pop != nil :%v", v)
			}
			s.Push("a")
			s.Push("a")
			s.Push("a")
			s.Init()
			if s.Size() != 0 {
				t.Fatalf("after Push Init err,size!=0,%d", s.Size())
			}
			if v, ok := s.Pop(); ok {
				t.Fatalf("after Push Init Pop != nil :%v", v)
			}

			s.Init()
			// Push,Pop测试
			p := 1
			s.Push(p)
			if s.Size() != 1 {
				t.Fatalf("after Push err,size!=1,%d", s.Size())
			}

			if v, ok := s.Pop(); !ok || v != p {
				t.Fatalf("Push want:%d, real:%v", p, v)
			}

			// size 测试
			s.Init()
			var n = 10
			var esum, dsum int
			for i := 0; i < n; i++ {
				if s.Push(i) {
					esum++
				}
			}
			if s.Size() != esum {
				t.Fatalf("Size want:%d, real:%v", esum, s.Size())
			}
			tk := time.NewTicker(time.Second * 5)
			defer tk.Stop()
			exit := false
			for !exit {
				select {
				case <-tk.C:
					t.Fatalf("size Pop timeout,")
					exit = true
				default:
					_, ok := s.Pop()
					if ok {
						dsum++
						tk.Reset(time.Second)
					}
					if s.Size() == 0 {
						exit = true
					}
				}
			}
			if dsum != esum {
				t.Fatalf("Size enqueue:%d, dequeue:%d,size:%d", esum, dsum, s.Size())
			}

			// 储存顺序测试,数组队列可能满
			// stack顺序反过来
			s.Init()
			array := [...]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
			sum := 0
			for i := range array {
				if s.Push(i) {
					//array[i] = i // queue用这种
					array[sum] = sum // stack用这种方式
					sum += 1
				}

			}
			for i := sum - 1; i >= 0; i-- {
				v, ok := s.Pop()
				if !ok || v != array[i] {
					t.Fatalf("array want:%d, real:%v,size:%d,%v", array[i], v, sum, ok)
				}
			}

			s.Init()
			// 空值测试
			var nullPtrs = unsafe.Pointer(nil)
			s.Push(nullPtrs)

			if v, ok := s.Pop(); !ok || nullPtrs != v {
				t.Fatalf("Push nil want:%v, real:%v", nullPtrs, v)
			}
			var null = new(interface{})
			s.Push(null)
			if v, ok := s.Pop(); !ok || null != v {
				t.Fatalf("Push nil want:%v, real:%v", null, v)
			}
		},
	})
}

func TestPush(t *testing.T) {
	const maxSize = 1 << 10
	var sum int64
	stackMap(t, stackStruct{
		setup: func(t *testing.T, s SInterface) {
		},
		perG: func(t *testing.T, s SInterface) {
			sum = 0
			for i := 0; i < maxSize; i++ {
				if s.Push(i) {
					atomic.AddInt64(&sum, 1)
				}
			}

			if s.Size() != int(sum) {
				t.Fatalf("TestConcurrentPush err,Push:%d,real:%d", sum, s.Size())
			}
		},
	})
}

func TestPop(t *testing.T) {
	const maxSize = 1 << 10
	var sum int64
	stackMap(t, stackStruct{
		setup: func(t *testing.T, s SInterface) {
		},
		perG: func(t *testing.T, s SInterface) {
			sum = 0
			for i := 0; i < maxSize; i++ {
				if s.Push(i) {
					atomic.AddInt64(&sum, 1)
				}
			}

			var dsum int64
			for i := 0; i < maxSize; i++ {
				_, ok := s.Pop()
				if ok {
					atomic.AddInt64(&dsum, 1)
				}
			}

			if int64(s.Size())+dsum != sum {
				t.Fatalf("TestPop err,Push:%d,Pop:%d,size:%d", sum, dsum, s.Size())
			}
		},
	})
}

func TestConStackInit(t *testing.T) {
	const maxGo = 4
	var timeout = time.Second * 5

	stackMap(t, stackStruct{
		setup: func(t *testing.T, s SInterface) {
		},
		perG: func(t *testing.T, s SInterface) {
			var wg sync.WaitGroup
			ctx, cancle := context.WithTimeout(context.Background(), timeout)

			for i := 0; i < maxGo; i++ {
				wg.Add(1)
				go func(ctx context.Context) {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							s.Pop()
							time.Sleep(time.Millisecond)
						}
					}
				}(ctx)
				wg.Add(1)
				go func(ctx context.Context) {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							s.Push(1)
						}
					}
				}(ctx)
				wg.Add(1)
				go func(ctx context.Context) {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							s.Init()
							time.Sleep(time.Millisecond * 10)
						}
					}
				}(ctx)
			}
			time.Sleep(1 * time.Second)
			cancle()
			wg.Wait()
			size := s.Size()
			sum := 0
			for {
				_, ok := s.Pop()
				if !ok {
					break
				}
				sum++
			}
			if size != sum {
				t.Fatalf("Init Concurrent err,real:%d,size:%d,ret:%d", sum, size, s.Size())
			}
		},
	})
}

func TestConcurrentPush(t *testing.T) {
	const maxGo, maxNum = 4, 1 << 8
	stackMap(t, stackStruct{
		setup: func(t *testing.T, s SInterface) {

		},
		perG: func(t *testing.T, s SInterface) {
			var wg sync.WaitGroup
			var esum int64
			for i := 0; i < maxGo; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < maxNum; i++ {
						if s.Push(i) {
							atomic.AddInt64(&esum, 1)
						}
					}
				}()
			}
			wg.Wait()
			if int64(s.Size()) != esum {
				t.Fatalf("TestConcurrentPush err,Push:%d,real:%d", esum, s.Size())
			}
		},
	})
}

func TestConcurrentPop(t *testing.T) {
	const maxGo, maxNum = 4, 1 << 20
	const maxSize = maxGo * maxNum

	stackMap(t, stackStruct{
		setup: func(t *testing.T, s SInterface) {
		},
		perG: func(t *testing.T, s SInterface) {
			var wg sync.WaitGroup
			var sum int64
			var PushSum int64
			for i := 0; i < maxSize; i++ {
				if s.Push(i) {
					PushSum += 1
				}
			}

			for i := 0; i < maxGo; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for s.Size() > 0 {
						_, ok := s.Pop()
						if ok {
							atomic.AddInt64(&sum, 1)
						}
					}
				}()
			}
			wg.Wait()

			if sum+int64(s.Size()) != int64(PushSum) {
				t.Fatalf("TestConcurrentPush err,Push:%d,Pop:%d,size:%d", PushSum, sum, s.Size())
			}
		},
	})
}

func TestConcurrentPushPop(t *testing.T) {
	const maxGo, maxNum = 4, 1 << 10
	stackMap(t, stackStruct{
		setup: func(t *testing.T, s SInterface) {
			// if _, ok := s.(*UnsafeQueue); ok {
			// 	t.Skip("UnsafeQueue can not test concurrent.")
			// }
		},
		perG: func(t *testing.T, s SInterface) {
			var PopWG sync.WaitGroup
			var PushWG sync.WaitGroup

			exit := make(chan struct{}, maxGo)

			var sumPush, sumPop int64
			for i := 0; i < maxGo; i++ {
				PushWG.Add(1)
				go func() {
					defer PushWG.Done()
					for j := 0; j < maxNum; j++ {
						if s.Push(j) {
							atomic.AddInt64(&sumPush, 1)
						}
					}
				}()
				PopWG.Add(1)
				go func() {
					defer PopWG.Done()
					for {
						select {
						case <-exit:
							return
						default:
							_, ok := s.Pop()
							if ok {
								atomic.AddInt64(&sumPop, 1)
							}
						}
					}
				}()
			}
			PushWG.Wait()
			close(exit)
			PopWG.Wait()
			exit = nil

			if sumPop+int64(s.Size()) != sumPush {
				t.Fatalf("TestConcurrentPushPop err,Push:%d,Pop:%d,instack:%d", sumPush, sumPop, s.Size())
			}
		},
	})
}
