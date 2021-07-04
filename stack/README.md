# Stack

-----

栈(`stack`)是非常常用的先进后出(**FILO**)数据结构，它只允许在队列的前端进行入栈出栈(`Push,Pop`)操作，[**lock-free**][1]的 `stack` 算法都是通过`CAS`操作实现的。

使用 `Go` 实现**栈**算法，队列包有以下几种队列：

1. 单锁链表，切片栈。
3. 无锁链表，切片栈。

| struct名 | 说明                                 | 使用场景                                                 |
| -------- | ------------------------------------ | -------------------------------------------------------- |
| SAStack  | 单锁切片，有界限，默认:DefaultSize。 | 简单并发不高，能预测多少的情况。栈内元素保持很少或在空。 |
| SLStack  | 单锁链表，无界限。                   | 简单并发不高，无法预测栈内元素有多少的情况。             |
| LLStack  | 无锁链表，无界限，默认:DefaultSize。 | 高频读写，无法预测栈内元素多少的情况。                   |
| LAStack  | 无锁切片，有界限，默认:DefaultSize。 | 连续读，连续写很多的场景。                               |

总体性能大概：LL>LA>SL>SA

Stack接口：

```go
type Stack interface {
	Push(i interface{}) bool
	Pop() (val interface{}, ok bool)
}
```

**Push：**

将val加入栈顶，返回是否成功。

**Pop：**

取出栈顶val，返回val和是否成功，如果不成功，val为nil。



-----




[1]: https://www.cs.rochester.edu/u/scott/papers/1996_PODC_queues.pdf

