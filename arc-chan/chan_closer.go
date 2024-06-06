/*
Author ：zaniu(zzaniu@126.com)
Time   ：2024/6/5 15:06
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
    "github.com/Zzaniu/tool/zlog"
    "sync/atomic"
)

type (
    ArcChan interface {
        Clone() ArcChan
        Close()
    }

    emptyArcChanInner struct {
        num      atomic.Int64
        isClosed atomic.Bool
        c        chan struct{}
    }

    EmptyArcChan struct {
        inner emptyArcChanInner
    }
)

// Clone 复制 chanCloser, 这里只是增加引用计数
func (e *emptyArcChanInner) Clone() ArcChan {
    if e.isClosed.Load() {
        return nil
    }
    if e.num.Add(1) <= 0 {
        zlog.Fatal("copy closed chanCloser")
    }
    return e
}

// Close 关闭 chan, 当且仅当所有引用都关闭时才真正关闭 chan
func (e *emptyArcChanInner) Close() {
    v := e.num.Add(-1)
    if v == -1 {
        close(e.c)
    }
    if v < -1 {
        zlog.Fatal("close closed chanCloser")
    }
}

// WaitClose 等待 chan 真正的关闭.
// 特别注意: 这里可能永远等不到返回, 因为引用计数可能一直不为 0.
func (e *emptyArcChanInner) closeAndWait() {
    // 这两行代码是有必要的, 我需要一个标志来判断是否需要真正关闭 chan
    e.isClosed.Store(true)
    e.Close()
    for range e.c {
    }
}

func NewEmptyArcChan() EmptyArcChan {
    return EmptyArcChan{inner: emptyArcChanInner{c: make(chan struct{})}}
}

func (e *EmptyArcChan) Clone() ArcChan {
    return e.inner.Clone()
}

// CloseAndWait 等待 chan 真正的关闭
func (e *EmptyArcChan) CloseAndWait() {
    e.inner.closeAndWait()
}
