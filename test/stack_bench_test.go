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

	"github.com/min1324/data/stack"
)

type mapStack string

const (
	opPush = mapStack("Push")
	opPop  = mapStack("Pop")
)

var mapStacks = [...]mapStack{opPush, opPop}

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
const stackMaxSize = 1 << 24 // queue max size
const prevPushSize = 1 << 20 // queue previous Push

func randStackCall(m SInterface) {
	op := mapStacks[rand.Intn(len(mapStacks))]
	switch op {
	case opPush:
		m.Push(1)
	case opPop:
		m.Pop()
	default:
		panic("invalid mapStack")
	}
}

type benchS struct {
	setup func(*testing.B, SInterface)
	perG  func(b *testing.B, pb *testing.PB, i int, m SInterface)
}

func benchSMap(b *testing.B, benchS benchS) {
	for _, m := range [...]SInterface{
		// // stack
		// &stack.LLStack{},
		// &stack.SAStack{},
		// &stack.SLStack{},
	} {
		b.Run(fmt.Sprintf("%T", m), func(b *testing.B) {
			m = reflect.New(reflect.TypeOf(m).Elem()).Interface().(SInterface)
			m.Init()

			if q, ok := m.(*stack.SAStack); ok {
				q.InitWith(stackMaxSize)
			}
			if benchS.setup != nil {
				benchS.setup(b, m)
			}

			b.ResetTimer()

			var i int64
			b.RunParallel(func(pb *testing.PB) {
				id := int(atomic.AddInt64(&i, 1) - 1)
				benchS.perG(b, pb, (id * b.N), m)
				m.Init()
			})
			// free

		})
	}
}

func BenchmarkPush(b *testing.B) {
	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
			for ; pb.Next(); i++ {
				m.Push(i)
			}
		},
	})
}

func BenchmarkPop(b *testing.B) {
	// 由于预存的数量<出队数量，无法准确测试dequeue
	const prevsize = 1 << 20
	benchSMap(b, benchS{
		setup: func(b *testing.B, m SInterface) {
			for i := 0; i < prevsize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
			for ; pb.Next(); i++ {
				m.Pop()
			}
		},
	})
}

func BenchmarkMostlyPush(b *testing.B) {
	const bit = 4
	const mark = 1<<bit - 1
	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {
			for i := 0; i < prevPushSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
			for ; pb.Next(); i++ {
				if mark == 0 {
					m.Pop()
				} else {
					m.Push(i)
				}
			}
		},
	})
}

func BenchmarkMostlyPop(b *testing.B) {
	const bit = 4
	const mark = 1<<bit - 1
	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {
			for i := 0; i < prevPushSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
			for ; pb.Next(); i++ {
				if i&mark == 0 {
					m.Push(i)
				} else {
					m.Pop()
				}
			}
		},
	})
}

func BenchmarkStackBalance(b *testing.B) {

	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
			for ; pb.Next(); i++ {
				m.Push(1)
				m.Pop()
				// b.Log("Push:", m.Size(), m.Push(i+10), m.Size(), i)
				// size := m.Size()
				// v, ok := m.Pop()
				// b.Logf("pop:%d,%v,%v,%d,%d", size, v, ok, m.Size(), i)
			}
		},
	})
}

func BenchmarkStackCollision(b *testing.B) {

	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {
			m.Push(1)
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
			for ; pb.Next(); i++ {
				if i&1 == 0 {
					// b.Log("Push:", m.Size(), m.Push(i), m.Size(), i)
					m.Pop()
				} else {
					m.Push(1)
					// size := m.Size()
					// v, ok := m.Pop()
					// b.Logf("pop:%d,%v,%v,%d,%d", size, v, ok, m.Size(), i)
				}
			}
		},
	})
}

func BenchmarkStackInterlace(b *testing.B) {

	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {
			for i := 0; i < prevPushSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
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

func BenchmarkStackConcurrentPop(b *testing.B) {

	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {

		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
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

func BenchmarkStackConcurrentPush(b *testing.B) {

	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {

		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
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

func BenchmarkStackConcurrentRand(b *testing.B) {
	const stackSize = 1 << 10
	rand.Seed(time.Now().Unix())

	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {

		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
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
						if (j^9)&1 == 0 {
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
						if (j^2)&1 == 0 {
							m.Push(j)
						} else {
							m.Pop()
						}
						atomic.AddUint64(&j, 1)
					}
				}
			}()
			for ; pb.Next(); i++ {
				if (j^3)&1 == 0 {
					m.Push(j)
				} else {
					m.Pop()
				}
				atomic.AddUint64(&j, 1)
			}
		},
	})
}

func BenchmarkStackConMulRand(b *testing.B) {
	rand.Seed(time.Now().Unix())

	const size = 1 << 10
	const mod = size - 1
	var random [size]int

	for i := range random {
		random[i] = rand.Intn(10) & 1
	}

	benchSMap(b, benchS{
		setup: func(_ *testing.B, m SInterface) {

		},
		perG: func(b *testing.B, pb *testing.PB, i int, m SInterface) {
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
					var j uint64
					for {
						select {
						case <-exit:
							return
						default:
							if random[atomic.AddUint64(&j, 1)&mod] == 0 {
								m.Push(j)
							} else {
								m.Pop()
							}
						}
					}
				}()
			}
			for ; pb.Next(); i++ {
				if random[i&mod] == 0 {
					m.Push(i)
				} else {
					m.Pop()
				}
			}
		},
	})
}
