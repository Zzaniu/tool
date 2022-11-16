/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/7/11 11:47
Desc   : 最不经常使用(最少次)淘汰算法(Least Frequently Used), LFU是淘汰一段时间内使用次数最少的(使用频率低的)

    ......................我佛慈悲......................

                           _oo0oo_
                          o8888888o
                          88" . "88
                          (| -_- |)
                          0\  =  /0
                        ___/`---'\___
                      .' \\|     |// '.
                     / \\|||  :  |||// \
                    / _||||| -卍-|||||- \
                   |   | \\\  -  /// |   |
                   | \_|  ''\---/''  |_/ |
                   \  .-\__  '-'  ___/-. /
                 ___'. .'  /--.--\  `. .'___
              ."" '<  `.___\_<|>_/___.' >' "".
             | | :  `- \`.;`\ _ /`;.`/ - ` : | |
             \  \ `_.   \_ __\ /__ _/   .-` /  /
         =====`-.____`.___ \_____/___.-`___.-'=====
                           `=---='

    ..................佛祖保佑, 永无BUG...................

*/

package lfu

import (
    "container/list"
    "sync"
)

type (
    Lfu interface {
        Set(k string, v interface{})
        Get(k string) (v interface{}, ok bool)
        Evict(n int)
        Size() int
    }

    lfu struct {
        sync.Mutex

        cap      int                // 容量
        kv       map[string]*kvItem // 存的 kvItem
        freqList *list.List         // 双链表, 存 freqNode, 就是频率还有为该频率的数据, 这些数据用 map(set) 存起来
    }

    kvItem struct {
        k      string
        v      interface{}
        parent *list.Element // 在双链表 freqList 中的 node(freqNode), 通过这个能知道频率
    }

    freqNode struct {
        freq  int // 访问的次数
        items map[*kvItem]interface{}
    }
)

var (
    placeholder = struct{}{}
)

func NewLfu(cap int) Lfu {
    return &lfu{
        cap:      cap,
        kv:       make(map[string]*kvItem),
        freqList: list.New(),
    }
}

// Set stores the given kv pair. If the cache has seen k before, the corresponding
// v will be updated and the frequency count be incremented. If the cache has never
// seen k before and full, the least frequently used k,v will be evicted.
func (l *lfu) Set(k string, v interface{}) {
    // 容量满了, 需要淘汰
    if l.cap > 0 && len(l.kv) >= l.cap {
        l.Evict(1)
    }

    l.Lock()
    defer l.Unlock()

    var item *kvItem

    // set 也算是一次访问, 所以如果该 key 之前就存在的话, 需要同时去更新频率
    if item, ok := l.kv[k]; ok {
        item.v = v
        l.increment(item)
        return
    }

    // 先把头节点取出来, 看下头节点是否为 nil 或者频率是不是为 1, 如果是则创建频率为1的新 node 从最前面插入,
    // 否则直接放到头节点的 items 中即可, 当然了, 肯定是需要放入 c.kv 中的啦
    front := l.freqList.Front()
    if front == nil || front.Value.(*freqNode).freq != 1 {
        node := &freqNode{
            freq:  1,
            items: map[*kvItem]interface{}{},
        }

        element := l.freqList.PushFront(node)

        item = &kvItem{
            k:      k,
            v:      v,
            parent: element,
        }

        node.items[item] = placeholder
    } else {
        item = &kvItem{
            k:      k,
            v:      v,
            parent: front,
        }

        front.Value.(*freqNode).items[item] = placeholder
    }
    l.kv[k] = item
    return
}

// Get returns the v related to k. The ok indicates whether it is found in cache.
func (l *lfu) Get(k string) (vv interface{}, ok bool) {
    l.Lock()
    defer l.Unlock()

    v, ok := l.kv[k]
    if !ok {
        return
    }

    vv = v.v

    l.increment(v) // 频率+1, 移动到新的 list
    return
}

// Evict evicts given number of items out of cache.
// 清理缓存, 把最近最少使用清掉
func (l *lfu) Evict(n int) {
    l.Lock()
    defer l.Unlock()

    if n <= 0 {
        return
    }

    i := 0

    for {
        // 如果已删除足够数量的数据, 或者 c.freqList 为 0 了(没数据了都), 那么直接退出
        if i == n || l.freqList.Len() == 0 {
            break
        }

        front := l.freqList.Front() // 获取到头节点, LFU 肯定是从低频率的开始清理
        frontNode := front.Value.(*freqNode)

        // 从头节点的 items 中随机删, 同时删除 c.kv 中的
        for item := range frontNode.items {
            delete(l.kv, item.k)
            delete(frontNode.items, item)
            i += 1
            if i == n {
                break
            }
        }

        // 如果 node 中的 items 长度未 0, 从 c.freqList 中删除该 node
        if len(frontNode.items) == 0 {
            l.freqList.Remove(front)
        }
    }
    return
}

// Size returns the number of items in cache
func (l *lfu) Size() int {
    l.Lock()
    defer l.Unlock()
    return len(l.kv)
}

// increment 频率+1, 移动到新的 node
func (l *lfu) increment(item *kvItem) {
    curr := item.parent                // 当前频率的 node
    currNode := curr.Value.(*freqNode) // 当前频率的 node 的值

    next := curr.Next() // 下一个节点
    var nextNode *freqNode
    if next != nil {
        nextNode = next.Value.(*freqNode)
    }

    // 如果下一个 node 是 nil 或者下一个 node 的频率不是当前频率+1的话, 需要新建一个 node 并插入,
    // 否则只需要把当前 item 从当前 items 删除并插入下一个 node 中的 items, 最后变更 item.parent 为新的即可
    if next == nil || (currNode.freq+1 != nextNode.freq) {
        node := &freqNode{
            freq: currNode.freq + 1,
            items: map[*kvItem]interface{}{
                item: placeholder,
            },
        }
        l.freqList.InsertAfter(node, curr)
    } else {
        nextNode.items[item] = placeholder
    }

    item.parent = curr.Next()

    // 从原来的节点的 items 中删除该 item
    delete(currNode.items, item)
    // 如果原来的节点的 items 为空, 直接删除原来的节点
    if len(currNode.items) == 0 {
		l.freqList.Remove(curr)
    }

    return
}
