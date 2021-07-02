package stack

/*
// Queue接口
type Stack interface {
	Push(i interface{}) bool
	Pop() (val interface{}, ok bool)
}

type XXStack struct {
	// 链表形式
	//
	// len uint32
	// top unsafe.Pointer
}

type XXNode struct {
	// 链表节点
	next  unsafe.Pointer
	p     interface{}

	// 额外可选
	// prev unsafe.Pointer
}

const (
	DefauleSize = 1 << 10		// 默认数组栈大小
	negativeOne = ^uint32(0)	// -1
)

栈满空条件：
名称				空						满
数组			top == 0				top == cap
切片			top == 0					无
链表			top == nil					无

slot:储存或者取出value时的节点node。
push:将slot.next=top,然后cas(top,top,slot)，成功则完成。
pop:cas(top,top,slot.next),成功则完成。

链表栈：
top指向栈顶,如果为nil,则栈空返回。

数组栈：
deID指向下次出队的node,enID指向下次入队的node,先操作，后移动ID.
*/
