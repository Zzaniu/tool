/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/1/11 11:47
Desc   : 最近最少使用(最长时间)淘汰算法(Least Recently Used), LRU是淘汰最长时间没有被使用的

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

package lru

import (
    "container/list"
    "github.com/Zzaniu/tool/safemap"
    "sync"
)

type (
    NewLruElement func(interface{}) *Element

    Lru interface {
        length() int
        push(*Element) *list.Element
        Get(interface{}, NewLruElement) *Element
        moveToFront(*list.Element)
        clean() interface{}
    }

    Element struct {
        Key interface{}
        Val interface{}
    }

    lru struct {
        sync.RWMutex
        lruMaxLength int
        list         *list.List
        m            safemap.SafeGcMap
    }
)

// Get 获取一个元素, 如果没有的话就用 NewLruElement 新建一个
// 如果已经超容量了, 就会触发清理, 将最近最少使用的元素移除(链表尾节点前移)
func (r *lru) Get(i interface{}, fn NewLruElement) *Element {
    r.RLock()
    element, exists := r.m.Get(i)
    if exists {
        r.moveToFront(element.(*list.Element))
        r.RUnlock()
        return element.(*list.Element).Value.(*Element)
    }
    r.RUnlock()

    r.Lock()
    element, exists = r.m.Get(i)
    if exists {
        r.moveToFront(element.(*list.Element))
        r.Unlock()
        return element.(*list.Element).Value.(*Element)
    }

    ele := fn(i)
    element = r.push(ele)
    r.m.Set(i, element)
    t := r.clean()
    if t != nil {
        r.m.Del(t.(*Element).Key)
    }
    r.Unlock()
    return ele
}

// Length 链表长度
func (r *lru) length() int {
    if r.list == nil {
        return 0
    }
    return r.list.Len()
}

// Push 存数据
func (r *lru) push(ele *Element) *list.Element {
    if r.list == nil {
        return nil
    }
    return r.list.PushFront(ele)
}

// MoveToFront 移动到链表头
func (r *lru) moveToFront(e *list.Element) {
    if r.list == nil {
        return
    }
    r.list.MoveToFront(e)
}

// Clean 执行清理, 删除将链表尾的数据(链表尾指针往前移动)
// 非线程安全的, 因为只在get中用, 同时get中又有锁, 所以这里没加锁
func (r *lru) clean() interface{} {
    if r.list == nil || r.length() <= r.lruMaxLength {
        return nil
    }
    v := r.list.Remove(r.list.Back())
    return v
}

func NewLru(lruMaxLength int, mThresholdFactor float32) Lru {
    if mThresholdFactor < 2 {
        mThresholdFactor = 2
    }
    return &lru{
        lruMaxLength: lruMaxLength,
        list:         list.New(),
		m:            safemap.NewAutoGcMap(lruMaxLength+1, int(mThresholdFactor*float32(lruMaxLength)+0.5)),
    }
}
