/*
Author ：zaniu(zzaniu@126.com)
Time   ：2024/6/5 20:37
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

package arc_chan

import (
    "testing"
    "time"
)

func TestNewEmptyArcChan(t *testing.T) {
    t.Run("is closed", func(t *testing.T) {
        x := NewEmptyArcChan()
        if !x.IsClosed() {
            t.Log("not closed")
        }
        x.CloseAndWait()
        if x.IsClosed() {
            t.Log("closed")
        }
    })
    t.Run("ArcChan", func(t *testing.T) {
        for i := 0; i < 1000; i++ {
            x := NewEmptyArcChan()
            for j := 0; j < 1000; j++ {
                closer := x.Clone()
                if closer == nil {
                    continue
                }
                go func(closer ArcChan) {
                    time.Sleep(time.Millisecond)
                    closer.Close()
                }(closer)
            }
            time.Sleep(time.Millisecond * 10)
            x.CloseAndWait()
        }
    })
    t.Run("ArcChanWithoutSleep", func(t *testing.T) {
        for i := 0; i < 1000; i++ {
            x := NewEmptyArcChan()
            for j := 0; j < 1000; j++ {
                closer := x.Clone()
                if closer == nil {
                    continue
                }
                go func(closer ArcChan) {
                    closer.Close()
                }(closer)
            }
            x.CloseAndWait()
        }
    })
}
