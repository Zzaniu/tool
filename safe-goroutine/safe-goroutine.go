/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/11/15 19:35
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
    "sync"
)

type (
    SafeGoroutine interface {
        Add(...func() error) // 添加任务
        Do()                 // 异步执行任务
        Wait() error         // 等待所有任务执行的结果, 如果发生错误, 返回第一个错误后直接结束, 不会再等待所有任务完成
    }
    safeGoroutine struct {
        sync.WaitGroup
        task       []task
        ctx        context.Context
        cancelFunc context.CancelFunc
        ch         chan error
        flg        int8
    }
    task struct {
        fn func() error
        ch chan error
    }
)

const (
    isAdd = 1
    isRan = 1 << iota
    isDone
)

var (
    onceErr   = fmt.Errorf("SafeGoroutine 不允许重复使用")
    noTaskErr = fmt.Errorf("无任何任务需要执行")
    noRunErr  = fmt.Errorf("请调用 SafeGoroutine.Do() 执行任务")
)

func (t *task) Done(ctx context.Context) <-chan error {
    go func() {
        defer func() {
            if e := recover(); e != nil {
                t.ch <- fmt.Errorf("%v", e)
            }
        }()
        select {
        case <-ctx.Done():
            close(t.ch)
        default:
            t.ch <- t.fn()
        }
    }()
    return t.ch
}

func (s *safeGoroutine) Add(fn ...func() error) {
    if s.flg&isRan == isRan {
        panic(onceErr)
    }
    s.flg = s.flg | isAdd
    if s.task == nil {
        s.task = make([]task, 0, len(fn))
    }
    for _, v := range fn {
        s.task = append(s.task, task{fn: v, ch: make(chan error)})
    }
}

func (s *safeGoroutine) Do() {
    if s.flg&isAdd != isAdd {
        panic(noTaskErr)
    }
    if s.flg&isRan == isRan {
        panic(onceErr)
    }
    s.flg = s.flg | isRan
    s.ch = make(chan error, len(s.task))
    for _, t := range s.task {
        s.WaitGroup.Add(1)
        go func(t task) {
            defer s.WaitGroup.Done()
            var err error
            select {
            case <-s.ctx.Done():
                err = s.ctx.Err()
            case err = <-t.Done(s.ctx):
            }
            if err != nil {
                s.ch <- err
            }
        }(t)
    }
}

func (s *safeGoroutine) Wait() (err error) {
    if s.flg&isRan != isRan {
        panic(noRunErr)
    }
    if s.flg&isDone == isDone {
        panic(onceErr)
    }
    s.flg = s.flg | isDone
    go func() {
        err = <-s.ch
        s.cancelFunc()
    }()
    s.WaitGroup.Wait()
    close(s.ch)
    return
}

func (s *safeGoroutine) DoAndWait() error {
    s.Do()
    return s.Wait()
}

func NewSafeGoroutine(ctx context.Context) SafeGoroutine {
    cancelCtx, cancelFunc := context.WithCancel(ctx)
    return &safeGoroutine{ctx: cancelCtx, cancelFunc: cancelFunc}
}
