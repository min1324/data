package queue

/*
// 接口
type Queue interface {
	Init()
	Size() int
	EnQueue(interface{}) bool
	DeQueue() (val interface{}, ok bool)
}

type XXQueue struct {
	// 链表形式
	//
	// once sync.Once
	// len  uint32
	// head unsafe.Pointer
	// tail unsafe.Pointer
	//
	// 额外可选项
	// popMu  sync.Mutex // pop操作锁
	// pushMu sync.Mutex // push操作锁

	// 数组形式
	//
	// len uint32
	// cap uint32
	// mod uint32
	// pushID uint32
	// popID  uint32
	// data   []snode
	//
	// 额外可选项
	// once	  sync.Once
	// popMu  sync.Mutex // pop操作锁
	// pushMu sync.Mutex // push操作锁
}

type XXNode struct {
	// 链表节点
	// stat标志node是否可用
	// poped_stat 状态时，已经取出，可以写入。push操作通过cas改变为pushing_stat,获得写入权限。
	// pushed_stat 状态时，已经写入,可以取出。pop操作通过cas改变为poping_stat,获得取出权限。
	// 额外状态。
	stat uint32
	next unsafe.Pointer
	p    interface{}

	// 额外可选
	// prev unsafe.Pointer
}

// node状态
const (
	poped_stat   uintptr = iota // 只能由push操作cas改变成pushing_stat, pop操作遇到，说明队列已空.
	pushing_stat                // 只能由push操作变成pushed_stat, pop操作遇到，说明队列已空.
	pushed_stat                 // 只能由pop操作cas变成poping_stat，push操作遇到，说明队列满了.
	poping_stat                 // 只能由pop操作变成poped_stat，push操作遇到，说明队列满了.
)

// 默认数组队列大小
const (
	defaultQueueSize = 1 << 10
	negativeOne = ^uint32(0) // -1
)

结构体名字：XXQueue
节点名字：xxNode

第一个字母:
single mutex	=>	S
dobule mutex	=>	D
lock-free		=>	L

第二个字母:
list	=>	L
array	=>	A
ring array	=>	R
*/
