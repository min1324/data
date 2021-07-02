# queue

-----

队列(`queue`)是非常常用的一个数据结构，它只允许在队列的前端（`head`）进行出队(`dequeue`)操作，而在队列的后端（`tail`）进行入队(`enqueue`)操作。[**lock-free**][1]的算法都是通过`CAS`操作实现的。

使用 `Go` 实现**队列**算法，队列包有以下几种队列：

1. 单锁链表，切片，环形队列。
2. 双锁链表，切片，环形队列。
3. 无锁链表，切片队列。

| struct名 | 说明                                 | 使用场景                                   |
| -------- | ------------------------------------ | ------------------------------------------ |
| SAQueue  | 单锁切片，无界限。                   | 简单并发不高，或者连续入队不多的情况。     |
| SLQueue  | 单锁链表，无界限。                   | 简单并发不高，无法预测连续入队多少的情况。 |
| SRQueue  | 单锁环形，有界限，默认:DefaultSize。 | 高频入队出队能预测多少的情况。             |
| DLQueue  | 双锁链表，无界限。                   | 连续入队，出队，但无法预测多少的情况。     |
| DRQueue  | 双锁环形，有界限，默认:DefaultSize。 | 频繁连续入队，出队，能预测多少的情况。     |
| LLQueue  | 无锁链表，无界限。                   | 高并发无法预测的情况。                     |
| LRQueue  | 无锁环形，有界限，默认:DefaultSize。 | 高并发，能预测最大容量情况。               |

Queue接口：

```go
type Queue interface {
	EnQueue(interface{}) bool
	DeQueue() (val interface{}, ok bool)
}
```

**EnQueue：**

将val加入队尾，返回是否成功。

**DeQueue：**

取出队头val，返回val和是否成功，如果不成功，val为nil。



-----




[1]: https://www.cs.rochester.edu/u/scott/papers/1996_PODC_queues.pdf

