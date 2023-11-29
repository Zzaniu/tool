package cache

import (
    "context"
    "time"
)

type (
    Option struct {
        Timeout       time.Duration
        RandomTimeout time.Duration
    }

    Opts func(*Option)

    Cache interface {
        Get(context.Context, string, func() (interface{}, error), ...Opts) (interface{}, error)
        MGet(context.Context, ...string) ([]interface{}, error)
        Del(context.Context, string) (bool, error)
        MDel(context.Context, ...string) ([]bool, error)
    }
)

func WithTimeout(timeout time.Duration) Opts {
    return func(opt *Option) {
        opt.Timeout = timeout
    }
}

func WithRandomTimeout(randomTimeout time.Duration) Opts {
    return func(opt *Option) {
        opt.RandomTimeout = randomTimeout
    }
}
