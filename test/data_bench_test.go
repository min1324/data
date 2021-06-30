package data_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/min1324/data/queue"
)

type mapOp string

const (
	opPush = mapOp("Push")
	opPop  = mapOp("Pop")
)

var mapOps = [...]mapOp{opPush, opPop}

/*
1<< 20~28
1048576		20
2097152		21
4194304		22
8388608		23
16777216	24
33554432	25
67108864	26
134217728	27
268435456	28
*/
const queueMaxSize = 1 << 24 // queue max size
const prevPushSize = 1 << 20 // queue previous push

func randCall(m SQInterface) {
	op := mapOps[rand.Intn(len(mapOps))]
	switch op {
	case opPush:
		m.Push(1)
	case opPop:
		m.Pop()
	default:
		panic("invalid mapOp")
	}
}

type bench struct {
	setup func(*testing.B, SQInterface)
	perG  func(b *testing.B, pb *testing.PB, i int, m SQInterface)
}

func benchMap(b *testing.B, bench bench) {
	for _, m := range [...]SQInterface{
		// queue
		// &UnsafeQueue{},
		// &queue.DLQueue{},
		&queue.DRQueue{},
		// &queue.LAQueue{},
		&queue.LLQueue{},
		// &queue.SAQueue{},
		// &queue.SLQueue{},
		// &queue.SRQueue{},
		&queue.Slice{},

		// // stack
		// &MutexStack{},
		// &stack.Stack{},
	} {
		b.Run(fmt.Sprintf("%T", m), func(b *testing.B) {
			m = reflect.New(reflect.TypeOf(m).Elem()).Interface().(SQInterface)
			m.Init()
			if q, ok := m.(*queue.LRQueue); ok {
				q.InitWith(queueMaxSize)
			}
			if q, ok := m.(*queue.DRQueue); ok {
				q.InitWith(queueMaxSize)
			}

			// setup
			if bench.setup != nil {
				bench.setup(b, m)
			}

			b.ResetTimer()

			var i int64
			b.RunParallel(func(pb *testing.PB) {
				id := int(atomic.AddInt64(&i, 1) - 1)
				bench.perG(b, pb, (id*b.N)%queueMaxSize, m)
			})
			// free
			m.Init()
		})
	}
}

func BenchmarkPush(b *testing.B) {
	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			for ; pb.Next(); i++ {
				m.Push(i)
			}
		},
	})
}

func BenchmarkPop(b *testing.B) {

	benchMap(b, bench{
		setup: func(b *testing.B, m SQInterface) {
			for i := 0; i < prevPushSize; i++ {
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

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < prevPushSize; i++ {
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
	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < prevPushSize; i++ {
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

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < prevPushSize; i++ {
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

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < prevPushSize; i++ {
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

func BenchmarkPushPopInterlace(b *testing.B) {

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			for i := 0; i < prevPushSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			j := 0
			for ; pb.Next(); i++ {
				j += (i & 1)
				if j&1 == 0 {
					m.Push(i)
				} else {
					m.Pop()
				}
			}
		},
	})
}

func BenchmarkConcurrentPushPop(b *testing.B) {

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			var wg sync.WaitGroup
			exit := make(chan struct{}, 1)
			defer func() {
				close(exit)
				wg.Wait()
				exit = nil
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-exit:
						return
					default:
						m.Push(1)
					}
				}
			}()
			for ; pb.Next(); i++ {
				m.Pop()
			}
		},
	})
}
func BenchmarkConcurrentPopPush(b *testing.B) {

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			var wg sync.WaitGroup
			exit := make(chan struct{}, 1)
			defer func() {
				close(exit)
				wg.Wait()
				exit = nil
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-exit:
						return
					default:
						m.Pop()
					}
				}
			}()
			for ; pb.Next(); i++ {
				m.Push(1)
			}
		},
	})
}

func BenchmarkConcurrentMostlyPush(b *testing.B) {

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			var wg sync.WaitGroup
			exit := make(chan struct{}, 1)
			defer func() {
				close(exit)
				wg.Wait()
				exit = nil
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-exit:
						return
					default:
						time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
						m.Pop()
					}
				}
			}()
			for ; pb.Next(); i++ {
				m.Push(1)
			}
		},
	})
}

func BenchmarkConcurrent(b *testing.B) {
	const stackSize = 1 << 10
	const push, pop = 128, 1

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
			if q, ok := m.(*queue.LRQueue); ok {
				q.InitWith(queueMaxSize)
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			var wg sync.WaitGroup
			exit := make(chan struct{}, 1)
			defer func() {
				close(exit)
				wg.Wait()
				exit = nil
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-exit:
						return
					default:
						m.Push(1)
					}
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-exit:
						return
					default:
						m.Pop()
					}
				}
			}()
			for ; pb.Next(); i++ {
				m.Size()
			}
		},
	})
}

func BenchmarkConcurrentRand(b *testing.B) {
	const stackSize = 1 << 10
	rand.Seed(time.Now().Unix())

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			var wg sync.WaitGroup
			exit := make(chan struct{}, 1)
			var j uint64
			defer func() {
				close(exit)
				wg.Wait()
				exit = nil
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-exit:
						return
					default:
						if j^7 == 0 {
							m.Push(j)
						} else {
							m.Pop()
						}
						atomic.AddUint64(&j, 1)
					}
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-exit:
						return
					default:
						if j^3 == 0 {
							m.Push(j)
						} else {
							m.Pop()
						}
						atomic.AddUint64(&j, 1)
					}
				}
			}()
			for ; pb.Next(); i++ {
				m.Size()
			}
		},
	})
}

func BenchmarkConcurrentMulRand(b *testing.B) {
	const stackSize = 1 << 10
	rand.Seed(time.Now().Unix())

	benchMap(b, bench{
		setup: func(_ *testing.B, m SQInterface) {
			if _, ok := m.(*UnsafeQueue); ok {
				b.Skip("UnsafeQueue can not test concurrent.")
			}
		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SQInterface) {
			exit := make(chan struct{}, 1)
			var wg sync.WaitGroup
			defer func() {
				close(exit)
				wg.Wait()
				exit = nil
			}()
			for g := int64(runtime.GOMAXPROCS(0)); g > 1; g-- {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-exit:
							return
						default:
							randCall(m)
						}
					}
				}()
			}
			for ; pb.Next(); i++ {
				randCall(m)
			}
		},
	})
}
