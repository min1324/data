package queue

/*
// Queue接口
type Queue interface {
	EnQueue(interface{}) bool
	DeQueue() (val interface{}, ok bool)
}

type DataQueue interface {
	Queue
	onceInit()
	Init()
	Size() int
	Full() bool
	Empty() bool
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
	next  unsafe.Pointer
	p     interface{}

	// 额外可选
	// prev unsafe.Pointer
}

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

slot:储存或者取出value时的节点node。
slot储存的val为空，则可以存入val，或者是DeQueue时为队列空。
val不为空时，可以取出val，或者是EnQueue时队列满。

链表队列：
head指向第一个出队的node,如果node的val为空，则可能由EnQueue操作，但未完成。
此时队列依旧认为空，直接返回。
tail指向下一个EnQueue的位置slot，先加入一个nilNode在slot.next,移动tail=slot.next。
然后在将value存入slot，完成加入操作。

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
