package data_test

import (
	"data/stack"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

type bench struct {
	setup func(*testing.B, SQInterface)
	perG  func(b *testing.B, pb *testing.PB, i int, m SQInterface)
}

func benchMap(b *testing.B, bench bench) {
	for _, m := range [...]SQInterface{
		// &UnsafeQueue{},
		// &MutexQueue{},
		// &queue.Queue{},
		// &MutexSlice{},
		// &queue.Slice{},
		&MutexStack{},
		&stack.Stack{},
	} {
		b.Run(fmt.Sprintf("%T", m), func(b *testing.B) {
			m = reflect.New(reflect.TypeOf(m).Elem()).Interface().(SQInterface)
			if bench.setup != nil {
				bench.setup(b, m)
			}

			b.ResetTimer()

			var i int64
			b.RunParallel(func(pb *testing.PB) {
				id := int(atomic.AddInt64(&i, 1) - 1)
				bench.perG(b, pb, id*b.N, m)
			})
		})
	}
}

func BenchmarkPush(b *testing.B) {
	const stackSize = 1 << 10

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < stackSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			for ; pb.Next(); i++ {
				m.Push(i)
			}
		},
	})
}

func BenchmarkPop(b *testing.B) {
	const stackSize = 1 << 10

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < stackSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			for ; pb.Next(); i++ {
				m.Pop()
			}
		},
	})
}

func BenchmarkMostlyPush(b *testing.B) {
	const stackSize = 1 << 10

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < stackSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			for ; pb.Next(); i++ {
				j := i % 4
				m.Push(i)
				if j == 0 {
					m.Pop()
				}
			}
		},
	})
}

func BenchmarkMostlyPop(b *testing.B) {
	const stackSize = 1 << 10

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < stackSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			for ; pb.Next(); i++ {
				j := i % 8
				if j == 0 {
					m.Push(i)
				}
				m.Pop()
			}
		},
	})
}

func BenchmarkPushPopBalance(b *testing.B) {
	const stackSize = 1 << 10

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < stackSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			for ; pb.Next(); i++ {
				m.Push(i)
				m.Pop()
			}
		},
	})
}

func BenchmarkPushPopCollision(b *testing.B) {
	const stackSize = 1 << 10

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < stackSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			for ; pb.Next(); i++ {
				if i%2 == 0 {
					m.Push(i)
				} else {
					m.Pop()
				}
			}
		},
	})
}

func BenchmarkConcurrentPushPop(b *testing.B) {
	const stackSize = 1 << 10

	exit := make(chan struct{}, 1)
	start := make(chan struct{}, 1)
	defer func() {
		close(start)
		close(exit)
		start = nil
		exit = nil
	}()

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			go func() {
				<-start
				for {
					select {
					case <-exit:
						return
					default:
						m.Push(1)
					}
				}
			}()
			start <- struct{}{}
			for ; pb.Next(); i++ {
				m.Pop()
			}
			exit <- struct{}{}
		},
	})
}

func BenchmarkConcurrentMostlyPush(b *testing.B) {
	const stackSize = 1 << 10

	exit := make(chan struct{}, 1)
	start := make(chan struct{}, 1)
	defer func() {
		close(start)
		close(exit)
		start = nil
		exit = nil
	}()

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			go func() {
				<-start
				for {
					select {
					case <-exit:
						return
					default:
						time.Sleep(time.Millisecond)
						m.Pop()
					}
				}
			}()
			start <- struct{}{}
			for ; pb.Next(); i++ {
				m.Push(i)
			}
			exit <- struct{}{}
		},
	})
}

func BenchmarkConcurrentMostlyPop(b *testing.B) {
	const stackSize = 1 << 10

	exit := make(chan struct{}, 1)
	start := make(chan struct{}, 1)
	defer func() {
		close(start)
		close(exit)
		start = nil
		exit = nil
	}()

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			go func() {
				<-start
				for {
					select {
					case <-exit:
						return
					default:
						time.Sleep(time.Millisecond)
						m.Push(1)
					}
				}
			}()
			start <- struct{}{}
			for ; pb.Next(); i++ {
				m.Pop()
			}
			exit <- struct{}{}
		},
	})
}
