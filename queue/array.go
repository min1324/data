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
	data []laNode
}

type laNode struct {
	// state有4个状态。
	// 这两个状态在pop操作时都表示为队列空。
	// poped_stat 状态时，已经取出，可以写入。push操作通过cas改变为pushing_stat,获得写入权限。
	// pushing_stat 状态时，正在写入。即队列为空。其他push操作不能更改，只能由push操作更改为pushed_stat,
	// 这两个状态在push操作时都表示为队列满。
	// pushed_stat 状态时，已经写入,可以取出。pop操作通过cas改变为poping_stat,获得取出权限。
	// poping_stat 状态时，正在取出。其他pop操作不能更改，并且只能由pop操作更改为poped_stat。
	state uintptr

	val unsafe.Pointer
}

const (
	poped_stat   uintptr = iota // 只能由push操作cas改变成pushing_stat, pop操作遇到，说明队列已空.
	pushing_stat                // 只能由push操作变成pushed_stat, pop操作遇到，说明队列已空.
	pushed_stat                 // 只能由pop操作cas变成poping_stat，push操作遇到，说明队列满了.
	poping_stat                 // 只能由pop操作变成poped_stat，push操作遇到，说明队列满了.
)

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
	data := make([]laNode, q.cap)
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
func (q *LAQueue) getSlot(id uintptr) *laNode {
	return &q.data[int(id&q.mod)]
}

// Set put i into queue
// if slot.stat == canPop, mean's not any more empty slot
// return
// push only done in can_set,
func (q *LAQueue) Set(i interface{}) (ok bool) {
	q.onceInit()
	var slot *laNode
	// 获取 slot
	for {
		// 获取最新 pushID,
		pushID := atomic.LoadUintptr(&q.pushID)
		slot = q.getSlot(pushID)
		stat := atomic.LoadUintptr(&slot.state)

		// 检测state,pushed_stat或者poping_stat表示队列满了。
		if stat == pushed_stat || stat == poping_stat {
			// TODO 是否需要写入缓冲区,或者扩容
			// queue full,
			return false
		}
		// 获取slot写入权限,将state变为:had_Set_PushStat
		if casUptr(&slot.state, poped_stat, pushing_stat) {
			// 成功获得slot
			// 先更新pushID,让其他push更快执行
			atomic.AddUintptr(&q.pushID, 1)
			break
		}
	}
	// 向slot写入数据
	slot.store(i)
	atomic.AddUintptr(&q.len, 1)

	// 更新state为pushed_stat.
	atomic.StoreUintptr(&slot.state, pushed_stat)
	return true
}

func (n *laNode) store(i interface{}) {
	atomic.StorePointer(&n.val, unsafe.Pointer(&i))
}

// 从队列中获取
func (q *LAQueue) Get() (e interface{}, ok bool) {
	q.onceInit()
	var slot *laNode
	// 获取 slot
	for {
		// 获取最新 popPID,
		popPID := atomic.LoadUintptr(&q.popID)
		slot = q.getSlot(popPID)
		stat := atomic.LoadUintptr(&slot.state)

		// 检测 pushStat 是否 can_set, 队列空了。
		if stat == poped_stat || stat == pushing_stat {
			// queue empty,
			return nil, false
		}

		// 获取slot取出权限,将state变为poping_stat
		if casUptr(&slot.state, pushed_stat, poping_stat) {
			// 成功取出slot
			// 先更新popPID,让其他pop更快执行
			atomic.AddUintptr(&q.popID, 1)
			break
		}
	}
	// 读取value
	e = slot.load()
	atomic.AddUintptr(&q.len, ^uintptr(0))

	// 更新pushStat为poped_stat.
	atomic.StoreUintptr(&slot.state, poped_stat)
	return e, true
}

func (n *laNode) load() interface{} {
	return *(*interface{})(n.val)
}

// Push put i into queue,
// wait until it success.
// use carefuly,if queue full, push still waitting.
func (q *LAQueue) Push(i interface{}) {
	for !q.Set(i) {
		runtime.Gosched()
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
	// return atomic.LoadUintptr(&q.len) == 0
	return q.popID == q.pushID
}

// 队列是否满
func (q *LAQueue) Full() bool {
	return q.pushID^q.cap == q.popID
	// return atomic.LoadUintptr(&q.len) == atomic.LoadUintptr(&q.cap)
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
