package queue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	had_Get_popStat uintptr = iota // 只能由 Set 操作将 popStat 变成 can_get
	can_Get_popStat                // 只能由 Get 操作将 popStat 变成 had_get， Set 操作遇到，说明队列满了.
)

const (
	can_Set_PushStat uintptr = iota // 只能由 Set 操作将 pushStat 变成 had_set， Get 操作遇到，说明队列已空.
	had_Set_PushStat                // 只能由 Get 操作将 pushStat 变成 can_set
)

const (
	normal_growStat uintptr = iota
	growing_growStat
)

const (
	defaultQueueSize = 1 << 8
)

type anode struct {

	// popStat 有两个状态，can 和 had
	// can 状态时，只能有一个线程通过 cas 改变为 had,
	// han 状态，其他线程不能更改，并且只能由 push 操作更改为 can
	popStat uintptr

	// popStat 有两个状态，can 和 had
	// can 状态时，只能有一个线程通过 cas 改变为 had,
	// han 状态，其他线程不能更改，并且只能由 pop 操作更改为 can
	pushStat uintptr

	// p 储存值
	p unsafe.Pointer
}

func newANode(i interface{}) *anode {
	return &anode{p: unsafe.Pointer(&i)}
}

func (n *anode) load() interface{} {
	return *(*interface{})(n.p)
}

func (n *anode) Store(i interface{}) {
	atomic.StorePointer(&n.p, unsafe.Pointer(&i))
}

// AQueue array queue
type AQueue struct {
	data []anode
	len  uintptr // queue len
	cap  uintptr // adjust to power of 2,
	mod  uintptr // cap-1

	pushID uintptr // data[pushID&mod].pushStat == can_set 时为可写入
	popPID uintptr // data[popPID&mod].popStat == can_get 时为可写入

	growStat uintptr // grow size statue,

	// 当 data 满时，push 将添加到 cache 队列。
	// push 先检测 cache 是否有数据，
	cache Queue
	once  sync.Once // only use for init
}

// onceInit initialize queue
// if use var q Queue statement struct
// user may forget Init Queue,
// so use a once func in Push to init queue.
func (q *AQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *AQueue) init() {
	if q.cap < 1 {
		q.cap = defaultQueueSize
	}
	q.mod = minMod(q.cap)
	q.cap = q.mod + 1
	q.data = make([]anode, q.cap)
}

func (q *AQueue) Init() {
	q.InitWith()
}

// InitWith 初始化长度为 cap 的queue,
// 如果未提供，则使用默认值: 1<<8
func (q *AQueue) InitWith(cap ...int) {
	if len(cap) > 0 && cap[0] > 0 {
		q.cap = uintptr(cap[0])
	}
	q.init()
}

// 当前元素个数
func (q *AQueue) Size() int {
	return int(q.len)
}

func (q *AQueue) Len() int {
	return int(q.len)
}

// 最大容量
func (q *AQueue) Cap() int {
	return int(q.cap)
}

// 队列是否空
func (q *AQueue) Empty() bool {
	// TODO edit logic
	return (q.popPID & q.mod) == (q.pushID & q.mod)
}

// 队列是否满
func (q *AQueue) Full() bool {
	// TODO edit logic
	return (q.popPID&q.mod + 1) == (q.pushID & q.mod)
}

func (q *AQueue) getSlot(id uintptr) *anode {
	return &q.data[int(id&q.mod)]
}

func (q *AQueue) grow() bool {
	return false
}

// Push put i into queue,
// wait until it success.
// use carefuly
func (q *AQueue) Push(i interface{}) {
	for _, ok := q.Set(i); !ok; {
		runtime.Gosched()
		_, ok = q.Set(i)
		// TODO need grow size ?
		// or add to temp queue
	}
}

// Set put i into queue
// if slot.stat == canPop, mean's not any more empty slot
// return
// push only done in can_set,
func (q *AQueue) Set(i interface{}) (e interface{}, ok bool) {
	q.onceInit()
	for {
		// 获取最新 pushID,
		pushID := atomic.LoadUintptr(&q.pushID)
		slot := q.getSlot(pushID)

		// 检测 popStat, can_get 表示队列满了。
		if atomic.LoadUintptr(&slot.popStat) == can_Get_popStat {
			// TODO 是否需要写入缓冲区
			// queue full,
			break
		}

		// 检测 pushStat 是否可以写入 can_set
		// cas 获得写入权限，pushStat 变为 han_push
		if !casUptr(&slot.pushStat, can_Set_PushStat, had_Set_PushStat) {
			// pushStat 是 had_set,已经被插入了
			continue
		}

		// 先更新 pushID,让其他线程更快执行
		atomic.AddUintptr(&q.pushID, 1)

		// 写入数据
		slot.Store(i)
		atomic.AddUintptr(&q.len, 1)

		// 更新 popStat 为 can_get
		atomic.StoreUintptr(&slot.popStat, can_Get_popStat)

		// exit loop
		return i, true
	}
	return nil, false
}

// Pop attempt to pop one element,
// if queue empty, it got nil.
func (q *AQueue) Pop() interface{} {
	e, _ := q.Get()
	return e
}

// 从队列中获取
func (q *AQueue) Get() (e interface{}, ok bool) {
	q.onceInit()
	for {
		// 获取最新 popPID,
		popPID := atomic.LoadUintptr(&q.popPID)
		slot := q.getSlot(popPID)

		// 检测 pushStat 是否 can_set, 队列空了。
		if atomic.LoadUintptr(&slot.pushStat) == can_Set_PushStat {

			// queue empty,
			break
		}

		// 检测 popStat 是否可以写入 can_get
		// cas 获得写入权限， popStat 变为 had_get
		if !casUptr(&slot.popStat, can_Get_popStat, had_Get_popStat) {
			// popStat 是 had_get, 已经被取出
			continue
		}

		// 先更新 popPID,让其他线程更快执行
		atomic.AddUintptr(&q.popPID, 1)

		// 读取数据
		value := slot.load()
		atomic.AddUintptr(&q.len, ^uintptr(0))

		// 更新 pushStat 为 can_set
		atomic.StoreUintptr(&slot.pushStat, can_Set_PushStat)

		// exit loop
		return value, true
	}
	return nil, false
}

func casUptr(addr *uintptr, old, new uintptr) bool {
	return atomic.CompareAndSwapUintptr(addr, old, new)
}

// 溢出环形计算需要，得出一个2的n次幂减1数（具体可百度kfifo）
func minMod(u uintptr) uintptr {
	//兼容0, as min as ,128->127 !255
	u -= 1
	u |= u >> 1
	u |= u >> 2
	u |= u >> 4
	u |= u >> 8
	u |= u >> 16
	return u
}
