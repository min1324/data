package stack_test

import (
	"data/stack"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
)

type Interface interface {
	Push(interface{})
	Pop() interface{}
}

type bench struct {
	setup func(*testing.B, Interface)
	perG  func(b *testing.B, pb *testing.PB, i int, m Interface)
}

func benchMap(b *testing.B, bench bench) {
	for _, m := range [...]Interface{
		&stack.Stack{},
		&MutexStack{},
	} {
		b.Run(fmt.Sprintf("%T", m), func(b *testing.B) {
			m = reflect.New(reflect.TypeOf(m).Elem()).Interface().(Interface)
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
	benchMap(b, bench{
		setup: func(_ *testing.B, m Interface) {
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m Interface) {
			for ; pb.Next(); i++ {
				m.Push(i)
			}
		},
	})
}

func BenchmarkPop(b *testing.B) {
	const stackSize = 1 << 10

	benchMap(b, bench{
		setup: func(_ *testing.B, m Interface) {
			for i := 0; i < stackSize; i++ {
				m.Push(i)
			}
		},

		perG: func(b *testing.B, pb *testing.PB, i int, m Interface) {
			for ; pb.Next(); i++ {
				m.Pop()
			}
		},
	})
}
