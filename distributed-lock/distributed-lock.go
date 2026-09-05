package distributed_lock

import (
    "context"
    "errors"
    "sync"
    "time"
)

var (
    LockOccupied = errors.New("the lock has been occupied")
    LockTimeout  = errors.New("the lock timeout")
    LockInUse    = errors.New("the lock instance is acquiring or has already been used")
    LockNotHeld  = errors.New("the lock has not been acquired")
)

type (
    DistributedLock interface {
        Lock(string, int, ...Options) error
        UnLock() error
    }

    // ManagedLock 增加支持 context 取消的清理方法和持锁状态监控。
    // Done 在开始释放锁或无法再保证持锁状态时关闭，不表示远端清理成功。
    // Err 记录首次结束持锁监控的原因；主动释放先触发结束时为 nil。
    // 清理结果由 UnLock 或 UnLockContext 返回，清理失败不会改写 Err。
    ManagedLock interface {
        DistributedLock
        UnLockContext(context.Context) error
        Done() <-chan struct{}
        Err() error
    }

    RetryStrategy interface {
        // NextBackoff 返回下一次重试前的等待时长。
        NextBackoff() time.Duration
    }

    Option struct {
        Retry RetryStrategy
        Lease bool
    }

    Options func(*Option)

    noRetry      time.Duration
    retreatRetry struct {
        mu  sync.Mutex
        num int
        max int
    }
)

func (r *retreatRetry) NextBackoff() time.Duration {
    r.mu.Lock()
    defer r.mu.Unlock()
    if r.num >= r.max {
        return 0
    }
    r.num++
    n := time.Duration(r.num)
    if n > time.Duration(1<<63-1)/time.Millisecond/n {
        return time.Duration(1<<63 - 1)
    }
    return n * n * time.Millisecond
}

func (n noRetry) NextBackoff() time.Duration {
    return time.Duration(n)
}

func NoRetry() RetryStrategy {
    return noRetry(0)
}

func RetreatRetry(max int) RetryStrategy {
    return &retreatRetry{max: max}
}

func WithRetry(retry RetryStrategy) Options {
    return func(o *Option) {
        // 内置重试配置可在多次 Lock 调用之间共享，每次调用创建独立状态。
        if r, ok := retry.(*retreatRetry); ok {
            o.Retry = NoRetry()
            if r != nil {
                o.Retry = RetreatRetry(r.max)
            }
            return
        }
        if retry == nil {
            o.Retry = NoRetry()
            return
        }
        o.Retry = retry
    }
}

// WithRetryFactory 为每次加锁创建独立的自定义重试状态。
// 带可变状态的自定义策略应通过工厂创建，避免通过 WithRetry 共享同一实例。
func WithRetryFactory(newRetry func() RetryStrategy) Options {
    return func(o *Option) {
        if newRetry != nil {
            o.Retry = newRetry()
        }
    }
}

func WithLease() Options {
    return func(o *Option) {
        o.Lease = true
    }
}
