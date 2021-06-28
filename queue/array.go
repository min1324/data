package queue

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	had_pop uintptr = iota // 只能由 push 操作将 popStat 变成 can_pop
	can_pop                // 只能由 pop  操作将 popStat 变成 had_pop
)

const (
	can_push uintptr = iota // 只能由 push 操作将 pushStat 变成 had_push
	had_push                // 只能由 pop  操作将 pushStat 变成 can_push
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

	pushID uintptr   // data[pushID&mod].pushStat == can_push 时为可写入
	popPID uintptr   // data[popPID&mod].popStat == can_pop 时为可写入
	once   sync.Once // only use for init
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
	q.mod = minCap(q.cap)
	q.cap = q.mod + 1
	q.data = make([]anode, q.cap)
}

func (q *AQueue) Init() {
	q.InitWith()
}

// 最多接受2个参数，分别为 cap,len
// 最大值值为 cap.
// 如果未提供，则使用默认值: 1<<8
func (q *AQueue) InitWith(arg ...int) {
	var scap int
	if len(arg) > 0 && arg[0] > 0 {
		scap = arg[0]
	}
	q.cap = uintptr(scap)
	q.init()
}

func (q *AQueue) Size() int {
	return int(q.len)
}

func (q *AQueue) Cap() int {
	return int(q.cap)
}

func (q *AQueue) Empty() bool {
	// TODO edit logic
	return (q.popPID & q.mod) == (q.pushID & q.mod)
}

func (q *AQueue) getSlot(id uintptr) *anode {
	return &q.data[int(id&q.mod)]
}

// Push put i into queue
// if slot.stat == canPop, mean's not any more empty slot
// return
// push only done in can_push,
func (q *AQueue) Push(i interface{}) {
	for {
		// 获取最新 pushID,
		pushID := atomic.LoadUintptr(&q.pushID)
		slot := q.getSlot(pushID)

		// 检测 popStat, can_pop 表示队列满了。
		if atomic.LoadUintptr(&slot.popStat) == can_pop {
			// queue full,
			break
		}

		// 检测 pushStat 是否可以写入 can_push
		// cas 获得写入权限，pushStat 变为 han_push
		if !casUptr(&slot.pushStat, can_push, had_push) {
			// pushStat 是 had_push,已经被插入了
			fmt.Println(slot.popStat, can_push, had_push)
			continue
		}

		// 先更新 pushID,让其他线程更快执行
		atomic.AddUintptr(&q.pushID, 1)

		// 写入数据
		slot.Store(i)
		atomic.AddUintptr(&q.len, 1)

		// 更新 popStat 为 can_pop
		atomic.StoreUintptr(&slot.popStat, can_pop)

		// exit loop
		break
	}
}

func (q *AQueue) Pop() interface{} {
	for {
		// 获取最新 popPID,
		popPID := atomic.LoadUintptr(&q.popPID)
		slot := q.getSlot(popPID)

		// 检测 pushStat 是否 can_push, 队列空了。
		if atomic.LoadUintptr(&slot.pushStat) == can_push {
			// queue empty,
			break
		}

		// 检测 popStat 是否可以写入 can_pop
		// cas 获得写入权限， popStat 变为 had_pop
		if !casUptr(&slot.popStat, can_pop, had_pop) {
			// popStat 是 had_pop, 已经被取出
			continue
		}

		// 先更新 popPID,让其他线程更快执行
		atomic.AddUintptr(&q.popPID, 1)

		// 读取数据
		value := slot.load()
		atomic.AddUintptr(&q.len, ^uintptr(0))

		// 更新 pushStat 为 can_push
		atomic.StoreUintptr(&slot.pushStat, can_push)

		// exit loop
		return value
	}
	return nil
}

func casUptr(addr *uintptr, old, new uintptr) bool {
	return atomic.CompareAndSwapUintptr(addr, old, new)
}

// 溢出环形计算需要，得出一个2的n次幂减1数（具体可百度kfifo）
func minCap(u uintptr) uintptr {
	u-- //兼容0, as min as ,128->127 !255
	u |= u >> 1
	u |= u >> 2
	u |= u >> 4
	u |= u >> 8
	u |= u >> 16
	return u
}

/*
var ErrEmpty = errors.New("queue is empty")
var ErrFull = errors.New("queue is full")
var ErrTimeout = errors.New("queue op. timeout")

type data struct {
	//isFull bool
	stat  uint32 //isFull(读写指针相同时有bug) 改为 stat , 0(可写), 1(写入中), 2(可读)，3(读取中)
	value interface{}
}

// BytesQueue .queque is a []byte slice
type BytesQueue struct {
	cap    uint32 //队列容量
	len    uint32 //队列长度
	ptrStd uint32 //ptr基准(cap-1)
	putPtr uint32 // queue[putPtr].stat must < 2
	getPtr uint32 // queue[getPtr].stat may < 2
	queue  []data //队列
}

//NewBtsQueue cap转换为的2的n次幂-1数,建议直接传入2^n数
//如出现 error0: the get pointer excess roll 说明需要加大队列容量(其它的非nil error 说明有bug)
//非 阻塞型的 put get方法 竞争失败时会立即返回
func NewBtsQueue(cap uint32) *BytesQueue {
	bq := new(BytesQueue)
	bq.ptrStd = minCap(cap)
	bq.cap = bq.ptrStd + 1
	bq.queue = make([]data, bq.cap)
	return bq
}

func minCap(u uint32) uint32 { //溢出环形计算需要，得出一个2的n次幂减1数（具体可百度kfifo）
	u-- //兼容0, as min as ,128->127 !255
	u |= u >> 1
	u |= u >> 2
	u |= u >> 4
	u |= u >> 8
	u |= u >> 16
	return u
}

//Len method
func (bq *BytesQueue) Len() uint32 {
	return atomic.LoadUint32(&bq.len)
}

//Empty method
func (bq *BytesQueue) Empty() bool {
	if bq.Len() > 0 {
		return false
	}
	return true
}

//Put method
func (bq *BytesQueue) Put(bs interface{}) (bool, error) {
	var putPtr, stat uint32
	var dt *data

	putPtr = atomic.LoadUint32(&bq.putPtr)

	if bq.Len() >= bq.ptrStd {
		return false, ErrFull
	}

	if !atomic.CompareAndSwapUint32(&bq.putPtr, putPtr, putPtr+1) {
		return false, nil
	}
	atomic.AddUint32(&bq.len, 1)
	dt = &bq.queue[putPtr&bq.ptrStd]

	for {
		stat = atomic.LoadUint32(&dt.stat) & 3
		if stat == 0 {
			//可写

			atomic.AddUint32(&dt.stat, 1)
			dt.value = bs
			atomic.AddUint32(&dt.stat, 1)
			return true, nil
		}
		runtime.Gosched()

	}

}

//Get method
func (bq *BytesQueue) Get() (interface{}, bool, error) {
	var getPtr, stat uint32
	var dt *data

	var bs interface{} //中间变量，保障数据完整性

	getPtr = atomic.LoadUint32(&bq.getPtr)

	if bq.Len() < 1 {
		return nil, false, ErrEmpty
	}

	if !atomic.CompareAndSwapUint32(&bq.getPtr, getPtr, getPtr+1) {
		return nil, false, nil
	}
	atomic.AddUint32(&bq.len, 4294967295) //^uint32(-1-1)==uint32(0)-uint32(1)
	dt = &bq.queue[getPtr&bq.ptrStd]

	for {
		stat = atomic.LoadUint32(&dt.stat)
		if stat == 2 {
			//可读
			atomic.AddUint32(&dt.stat, 1) // change stat to 读取中
			bs = dt.value
			dt.value = nil
			atomic.StoreUint32(&dt.stat, 0) //重置stat为0
			return bs, true, nil

		}
		runtime.Gosched()

	}

}

// PutWait 阻塞型put,ms 最大等待豪秒数,默认 1000
func (bq *BytesQueue) PutWait(bs interface{}, ms ...time.Duration) error {

	var start, end time.Time

	start = time.Now()
	end = start.Add(time.Millisecond * 1000)
	if len(ms) > 0 {
		end = end.Add(time.Millisecond * ms[0])
	}

	for {
		ok, err := bq.Put(bs)
		if ok {
			return nil
		}
		if ErrFull == err {
			time.Sleep(50 * time.Millisecond)
		}

		if time.Now().After(end) {
			return ErrTimeout
		}

	}

}

// GetWait 阻塞型get, ms为 等待毫秒 默认1000
func (bq *BytesQueue) GetWait(ms ...time.Duration) (interface{}, error) {

	var start, end time.Time

	start = time.Now()
	end = start.Add(time.Millisecond * 1000)
	if len(ms) > 0 {
		end = start.Add(time.Millisecond * ms[0])
	}

	for {
		value, ok, err := bq.Get()
		if ok {
			return value, nil
		}

		if ErrEmpty == err {
			time.Sleep(50 * time.Millisecond)
		}

		if time.Now().After(end) {
			return nil, ErrTimeout
		}

	}

}
*/
