package queue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

// LAQueue array queue
type LAQueue struct {
	// 声明队列后，如果没有调用Init(),队列不能使用
	// 为解决这个问题，加入一次性操作。
	// 能在队列声明后，调用push或者pop操作前，初始化队列。
	// 见 onceInit()
	once sync.Once

	len uintptr // 队列当前数据长度
	cap uintptr // 队列容量，自动向上调整至2^n
	mod uintptr // cap-1,即2^n-1,用作取slot: data[ID&mod]

	// 指向下次写入数据的位置:pushID&mod
	pushID uintptr

	// 指向取出数据的位置:popID&mod
	popID uintptr

	// 环形队列，大小必须是2的倍数。
	data []anode

	// growing_Stat 扩容中，normal_Stat 正常状态
	// TODO 暂停其他所有操作? 或者设置额外索引指向新数组.
	// growStat uintptr

	// // 当 data 满时，push 将添加到 cache 队列。
	// // push 先检测 cache 是否有数据，
	// cache Queue

	// // cacheStat 有4种状态
	// // empty_cacheStat 时，表示没有数据。
	// // hald_cacheStat 时表示有数据，push 操作先将数据加入 cache 队列里
	// // pop_cacheStat 表示正在 pop 操作，其他 pop 操作需要等待
	// // push_cacheStat 表示 push 正在迁移数据
	// cacheStat uintptr

}

type anode struct {
	// popStat有两个状态，can和had。
	// can 状态时，可以取出,即已经写入。pop操作通过cas改变为:had,获得读取权限。push操作时，表示队列满。
	// had 状态时，已经取出。其他pop操作不能更改，并且只能由push操作更改为:can
	popStat uintptr

	// pushStat有两个状态，can和had。
	// can 状态时，可以写入,即已经取出。push操作通过cas改变为:had,获得写入权限。pop操作时，表示队列空。
	// had 状态时，已经写入。其他pop操作不能更改，并且只能由push操作更改为:can
	pushStat uintptr

	val unsafe.Pointer
}

const (
	had_Get_popStat uintptr = iota // 只能由 Set 操作将 popStat 变成 can_get
	can_Get_popStat                // 只能由 Get 操作将 popStat 变成 had_get， Set 操作遇到，说明队列满了.
)

const (
	can_Set_PushStat uintptr = iota // 只能由 Set 操作将 pushStat 变成 had_set， Get 操作遇到，说明队列已空.
	had_Set_PushStat                // 只能由 Get 操作将 pushStat 变成 can_set
)

// func newANode(i interface{}) *anode {
// 	return &anode{p: unsafe.Pointer(&i)}
// }

const (
	defaultQueueSize = 1 << 10
)

// 一次性初始化
func (q *LAQueue) onceInit() {
	q.once.Do(func() {
		q.init()
	})
}

func (q *LAQueue) init() {
	if q.cap < 1 {
		q.cap = defaultQueueSize
	}
	q.popID = q.pushID
	q.mod = minMod(q.cap)
	q.cap = q.mod + 1
	q.len = 0
	data := make([]anode, q.cap)
	q.data = data
}

func (q *LAQueue) Init() {
	q.InitWith()
}

// InitWith 初始化长度为cap的queue,
// 如果未提供，则使用默认值: 1<<8
func (q *LAQueue) InitWith(cap ...int) {
	if len(cap) > 0 && cap[0] > 0 {
		q.cap = uintptr(cap[0])
	}
	q.init()
}

// 当前元素个数
func (q *LAQueue) Size() int {
	return int(q.len)
}

// 根据pushID,popID获取进队，出队对应的slot
func (q *LAQueue) getSlot(id uintptr) *anode {
	return &q.data[int(id&q.mod)]
}

// Set put i into queue
// if slot.stat == canPop, mean's not any more empty slot
// return
// push only done in can_set,
func (q *LAQueue) Set(i interface{}) (ok bool) {
	q.onceInit()
	var slot *anode
	// 获取 slot
	for {
		// 获取最新 pushID,
		pushID := atomic.LoadUintptr(&q.pushID)
		slot = q.getSlot(pushID)

		// 检测 popStat, can_get 表示队列满了。
		if atomic.LoadUintptr(&slot.popStat) == can_Get_popStat {
			// TODO 是否需要写入缓冲区,或者扩容
			// queue full,
			return false
		}

		// 检测pushStat是否可以写入:can_Set_PushStat
		// cas获得写入权限，将pushStat变为:had_Set_PushStat
		if casUptr(&slot.pushStat, can_Set_PushStat, had_Set_PushStat) {
			// pushStat是had_Set_PushStat,成功插入slot
			// 先更新 pushID,让其他线程更快执行
			atomic.AddUintptr(&q.pushID, 1)
			break
		}
	}
	// slot写入数据
	slot.store(i)
	atomic.AddUintptr(&q.len, 1)

	// 更新popStat为can_Get_popStat, Get()可以取出了
	atomic.StoreUintptr(&slot.popStat, can_Get_popStat)

	return true
}

func (n *anode) store(i interface{}) {
	atomic.StorePointer(&n.val, unsafe.Pointer(&i))
}

// 从队列中获取
func (q *LAQueue) Get() (e interface{}, ok bool) {
	q.onceInit()
	var slot *anode
	// 获取 slot
	for {
		// 获取最新 popPID,
		popPID := atomic.LoadUintptr(&q.popID)
		slot = q.getSlot(popPID)

		// 检测 pushStat 是否 can_set, 队列空了。
		if atomic.LoadUintptr(&slot.pushStat) == can_Set_PushStat {
			// queue empty,
			return nil, false
		}

		// 检测popStat是否可以取出:can_Get_popStat
		// cas获得写入权限，将popStat变为:had_Get_popStat
		if casUptr(&slot.popStat, can_Get_popStat, had_Get_popStat) {
			// popStat是had_Get_popStat,成功取出slot
			// 先更新 popPID,让其他线程更快执行
			atomic.AddUintptr(&q.popID, 1)
			break
		}
	}
	e = slot.load()
	// 读取数据
	atomic.AddUintptr(&q.len, ^uintptr(0))

	// 更新pushStat为can_Set_PushStat, Set()可以写入了
	atomic.StoreUintptr(&slot.pushStat, can_Set_PushStat)

	return e, true
}

func (n *anode) load() interface{} {
	return *(*interface{})(n.val)
}

// Push put i into queue,
// wait until it success.
// use carefuly,if queue full, push still waitting.
func (q *LAQueue) Push(i interface{}) {
	for ok := q.Set(i); !ok; {
		runtime.Gosched()
		ok = q.Set(i)
		// TODO need grow size ?
		// or add to temp queue
	}
}

// not use yet
func (q *LAQueue) grow() bool {
	// TODO grow queue data
	return false
}

// Pop attempt to pop one element,
// if queue empty, it got nil.
func (q *LAQueue) Pop() interface{} {
	e, _ := q.Get()
	return e
}

// queue's len
func (q *LAQueue) Len() int {
	return int(q.len)
}

// queue's cap
func (q *LAQueue) Cap() int {
	return int(q.cap)
}

// 队列是否空
func (q *LAQueue) Empty() bool {
	return atomic.LoadUintptr(&q.len) == 0
	//	return (q.popID & q.mod) == (q.pushID & q.mod)
}

// 队列是否满
func (q *LAQueue) Full() bool {
	return atomic.LoadUintptr(&q.len) == atomic.LoadUintptr(&q.cap)
	// return (q.popID&q.mod + 1) == (q.pushID & q.mod)
}

func casUptr(addr *uintptr, old, new uintptr) bool {
	return atomic.CompareAndSwapUintptr(addr, old, new)
}

// 溢出环形计算需要，得出2^n-1。(2^n>=u,具体可见kfifo）
func minMod(u uintptr) uintptr {
	u -= 1 //兼容0, as min as ,128->127 !255
	u |= u >> 1
	u |= u >> 2
	u |= u >> 4
	u |= u >> 8  // 32位类型已经足够
	u |= u >> 16 // 64位
	return u
}
