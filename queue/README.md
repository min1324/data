# queue

-----

一个使用 Go 实现 **lock-free** 算法的队列 。    

-----

队列(`queue`)是非常常用的一个数据结构，它只允许在队列的前端（`head`）进行出队(`dequeue`)操作，而在队列的后端（`tail`）进行入队(`enqueue`)操作。[**lock-free**][1]的算法都是通过`CAS`操作实现的。


[1]: https://www.cs.rochester.edu/u/scott/papers/1996_PODC_queues.pdf

