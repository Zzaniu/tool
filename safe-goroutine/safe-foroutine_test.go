/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/11/16 14:23
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

package safe_goroutine

import (
    "context"
    "fmt"
    "testing"
    "time"
)

func TestSafeGoroutine(t *testing.T) {
    ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
    defer cancelFunc()
    s := NewSafeGoroutine(ctx)
    s.Add(func() error {
        time.Sleep(time.Second)
        fmt.Println("task 1")
        return nil
    }, func() error {
        time.Sleep(time.Second * 2)
        fmt.Println("task 2")
        return nil
    }, func() error {
        time.Sleep(time.Millisecond * 100)
        // return fmt.Errorf("tsak3 error")
        return nil
    })
    s.Do()
    if err := s.Wait(); err != nil {
        panic(err)
    }
}

// go test .