/*
Author ：zaniu(zzaniu@126.com)
Time   ：2024/12/11 11:54
Desc   :

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
    "testing"
)

func TestNewLfu(t *testing.T) {
    t.Run("test lfu", func(t *testing.T) {
        for i := 0; i < 10000; i++ {
            lfuCache := NewLfu(3)
            lfuCache.Set("key1", "value1")
            lfuCache.Set("key2", "value2")
            lfuCache.Set("key3", "value3")

            _, ok := lfuCache.Get("key1")
            if !ok {
                panic("key1 should be exist")
            }
            _, ok = lfuCache.Get("key2")
            if !ok {
                panic("key2 should be exist")
            }
            lfuCache.Set("key4", "value4")
            _, ok = lfuCache.Get("key3")
            if ok {
                panic("key3 should be deleted")
            }
            _, ok = lfuCache.Get("key1")
            if !ok {
                panic("key1 should be exist")
            }
            for range 2 {
                _, ok = lfuCache.Get("key4")
                if !ok {
                    panic("key4 should be exist")
                }
            }
            lfuCache.Evict(1)
            _, ok = lfuCache.Get("key2")
            if ok {
                panic("key2 should be deleted")
            }
        }
    })
}
