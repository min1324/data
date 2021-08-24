# Set
## 集合基本操作
### 位运算
&   位与
 |   位或
 ^   异或
&^   位与非
<<   左移
>>   右移

### 集合操作
Intersect: 交集，属于set或属于others的元素为元素的集合。
Diff: 差集，属于set且不属于others的元素为元素的集合。
Union: 并集，属于set或属于others的元素为元素的集合。
Complement: 补集，(前提: set应当为full的子集)属于全集full不属于集合set的元素组成的集合。如果给定的full集合不是set的全集时，返回full与set的差集.