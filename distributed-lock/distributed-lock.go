package distributed_lock

import (
    "errors"
    "time"
)

var (
    LockOccupied = errors.New("the lock has been occupied")
    LockTimeout  = errors.New("the lock timeout")
)

type (
    DistributedLock interface {
        Lock(string, int, ...Options) error
        UnLock() error
    }

    RetryStrategy interface {
        // NextBackoff returns the next backoff duration.
        NextBackoff() time.Duration
    }

    Option struct {
        Retry RetryStrategy
        Lease bool
    }

    Options func(*Option)

    noRetry      time.Duration
    retreatRetry struct {
        num int
        max int
    }
)

func (r *retreatRetry) NextBackoff() time.Duration {
    r.num++
    if r.num > r.max {
        return 0
    }
    return time.Duration(r.num*r.num) * time.Millisecond
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
        o.Retry = retry
    }
}

func WithLease() Options {
    return func(o *Option) {
        o.Lease = true
    }
}
