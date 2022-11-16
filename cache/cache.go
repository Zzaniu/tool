package cache

import "time"

type (
    Option struct {
        Timeout       time.Duration
        RandomTimeout time.Duration
    }

    Opts func(*Option)

    Cache interface {
        Get(string, func() (string, error), ...Opts) (string, error)
        MGet(...string) ([]interface{}, error)
        Del(string) (bool, error)
        MDel(...string) ([]bool, error)
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
