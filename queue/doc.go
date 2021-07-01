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
	// deMu sync.Mutex // pop操作锁
	// enMu sync.Mutex // push操作锁

	// 数组形式
	// once sync.Once
	// len  uint32
	// cap  uint32
	// mod  uint32
	// deID uint32 // 指向下次取出数据的位置:deID&mod
	// enID uint32 // 指向下次写入数据的位置:enID&mod
	// data []node
	//
	// 额外可选项
	// deMu sync.Mutex // pop操作锁
	// enMu sync.Mutex // push操作锁
}

type XXNode struct {
	// 链表节点
	// stat标志node是否可用
	// geted_stat 状态时，已经取出，可以写入。EnQUeue操作通过cas改变为setting_stat,获得写入权限。
	// seted_stat 状态时，已经写入，可以取出。DeQueue操作通过cas改变为getting_stat,获得取出权限。
	// 额外状态。
	state uint32
	next  unsafe.Pointer
	p     interface{}

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

const (
	DefauleSize = 1 << 10		// 默认数组队列大小
	negativeOne = ^uint32(0)	// -1
)

队列满空条件：
名称				空						满
数组			len == 0				len == cap
切片			len == 0					无
链表		q.head == q.tail				无
环形		q.deID == q.enID		q.enID^q.cap == q.deID

哨兵位：
链表队列都有一个node哨兵位，head指向哨兵位，不存数据，或者是上次dequeue的node.
出队操作，移动head到node下一个，然后释放node.返回head指向的新node值
入队操作，tail.next记录新node,然后移动tail到新node.

数组队列：
deID指向下次出队的node,enID指向下次入队的node,先操作，后移动ID.

结构体名字：XXQueue
第一个字母:
single mutex	=>	S
dobule mutex	=>	D
lock-free		=>	L

第二个字母:
list	=>	L	// 链表
array	=>	A	// 数组
ring 	=>	R	// 环形数组
*/
