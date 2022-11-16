package sre

import (
    "errors"
    "github.com/Zzaniu/tool/window"
    "math"
    "math/rand"
    "sync"
    "time"
)

var ErrNotAllowed = errors.New("circuitbreaker: not allowed for circuit open")

// SreBreaker google sre 弹性熔断器
type (
    SreBreaker struct {
        k      float64
        policy window.Rolling

        request  uint64 // 如果请求总数小于这个数的话, 不开启熔断. 一般是几个
        r        *rand.Rand
        randLock sync.Mutex
    }
    Opt struct {
        k       float64
        request uint64
        opts    []window.Option
    }

    Option func(*Opt)
)

// Allow 是否允许改请求通过, 如果返回 error 说明被熔断了
func (s *SreBreaker) Allow() error {
    accept, reqTotal := s.policy.Summary()
    requests := s.k * float64(accept)
    if reqTotal < s.request || float64(reqTotal) < requests {
        // 如果 s.state 等于 StateOpen 则更新为 StateClosed
        return nil
    }
    dr := math.Max(0, (float64(reqTotal)-requests)/float64(reqTotal+1))
    if dr <= 0 {
        return nil
    }
    if s.trueOnProba(dr) {
        return ErrNotAllowed
    }
    return nil
}

// MarkSuccess 标记成功
func (s *SreBreaker) MarkSuccess() {
    s.policy.Add(1)
}

// MarkFailed 标记失败
func (s *SreBreaker) MarkFailed() {
    s.policy.Add(0)
}

func (s *SreBreaker) trueOnProba(proba float64) (truth bool) {
    s.randLock.Lock()
    truth = s.r.Float64() < proba // 这里是伪随机数有什么问题吗？个人感觉是没问题的
    s.randLock.Unlock()
    return
}

func WithK(k float64) Option {
    return func(opt *Opt) {
        opt.k = k
    }
}

func WithRequest(request uint64) Option {
    return func(opt *Opt) {
        opt.request = request
    }
}

func WithPolicyOption(policy ...window.Option) Option {
    return func(opt *Opt) {
        opt.opts = append(opt.opts, policy...)
    }
}

func NewSreBreaker(opts ...Option) *SreBreaker {
    opt := Opt{
        k:       1 / 0.8,
        request: 5,
    }

    for _, o := range opts {
        o(&opt)
    }

    return &SreBreaker{
        k:       opt.k, // 10个里面允许有2个异常
        request: opt.request,
        r:       rand.New(rand.NewSource(time.Now().UnixNano())),
        policy:  window.NewRollingPolicy(opt.opts...),
    }

}
