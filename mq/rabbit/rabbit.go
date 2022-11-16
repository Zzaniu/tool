package rabbit

import (
    "errors"
    "github.com/streadway/amqp"
    "time"
)

type (
    RbInfo struct {
        Addr             string
        ExchangeName     string
        QueueName        string
        RouteKey         string
        DeadExchangeName string
        DeadQueueName    string
        DeadRouteKey     string
        opts             options
    }

    ExchangeDeclareOpt struct {
        ExchangeType string
        AutoDelete   bool
        Internal     bool
        NoWait       bool
        Arguments    amqp.Table
    }

    QueueDeclareOpt struct {
        AutoDelete bool
        Exclusive  bool
        NoWait     bool
        Arguments  amqp.Table
    }

    QueueBindOpt struct {
        NoWait    bool
        Arguments amqp.Table
    }

    ConsumeOpt struct {
        NoLocal   bool
        NoAck     bool
        Exclusive bool
        NoWait    bool
        Arguments amqp.Table
    }

    options struct {
        ReconnectDelay time.Duration
        ResendDelay    time.Duration
        Durable        bool
        DeadDurable    bool
        ExOpt          ExchangeDeclareOpt
        QOpt           QueueDeclareOpt
        QBind          QueueBindOpt
        DeadExOpt      ExchangeDeclareOpt
        DeadQOpt       QueueDeclareOpt
        DeadQBind      QueueBindOpt
        ConsumeOpt     ConsumeOpt
    }
)

const (
    ReconnectDelay = 10 // 连接断开后多久重连
    ResendDelay    = 3  // 消息发送超时时间
)

var (
    ParamsError   = errors.New("地址、路由和队列不能为空")
    CallBackError = errors.New("消费回调函数不能为空")
)

func Init(rbInfo *RbInfo) {
    rbInfo.opts.ReconnectDelay = time.Second * ReconnectDelay // 连接断开后每隔 10s 进行重连
    rbInfo.opts.ResendDelay = time.Second * ResendDelay       // 消息发送超时时间 3s
    rbInfo.opts.ExOpt.ExchangeType = amqp.ExchangeDirect      // 默认路由模式
    rbInfo.opts.DeadExOpt.ExchangeType = amqp.ExchangeFanout  // 死信队列默认广播模式, 其实也可以用路由模式
    rbInfo.opts.Durable = true                                // 默认持久化
    // 如果有设置死信队列, 需要设置对应的 Arguments
    if len(rbInfo.DeadExchangeName) > 0 && len(rbInfo.DeadQueueName) > 0 {
        rbInfo.opts.DeadDurable = true
        rbInfo.opts.QOpt.Arguments = make(amqp.Table, 1)
        rbInfo.opts.QOpt.Arguments["x-dead-letter-exchange"] = rbInfo.DeadExchangeName
    }
}

func NewRabbitProduct(rbInfo RbInfo, callBack func(amqp.Delivery), opts ...DialOption) (*RbMqClient, error) {
    if len(rbInfo.Addr) == 0 || len(rbInfo.ExchangeName) == 0 || len(rbInfo.QueueName) == 0 {
        return nil, ParamsError
    }
    Init(&rbInfo)
    for _, opt := range opts {
        opt.apply(&rbInfo.opts)
    }
    product := &RbMqClient{
        rbInfo:              rbInfo,
        done:                make(chan struct{}),
        coonNotifyConnected: make(chan struct{}),
    }
    if callBack != nil {
        product.callBack = callBack
    }
    return product, nil
}

type DialOption interface {
    apply(*options)
}

type funcDialOption struct {
    f func(*options)
}

func (fdo *funcDialOption) apply(do *options) {
    fdo.f(do)
}

func newFuncDialOption(f func(*options)) *funcDialOption {
    return &funcDialOption{
        f: f,
    }
}

// WithReconnectDelay 链接超时时间
func WithReconnectDelay(reconnectDelay time.Duration) DialOption {
    return newFuncDialOption(func(o *options) {
        o.ReconnectDelay = reconnectDelay
    })
}

// WithResendDelay 发送超时时间
func WithResendDelay(resendDelay time.Duration) DialOption {
    return newFuncDialOption(func(o *options) {
        o.ResendDelay = resendDelay
    })
}

// WithDurable 是否持久化
func WithDurable(durable bool) DialOption {
    return newFuncDialOption(func(o *options) {
        o.Durable = durable
    })
}

// WithDeadDurable 死信队列是否持久化
func WithDeadDurable(durable bool) DialOption {
    return newFuncDialOption(func(o *options) {
        o.DeadDurable = durable
    })
}

// WithExchangeOpt 交换机配置
func WithExchangeOpt(exchangeOpt ExchangeDeclareOpt) DialOption {
    return newFuncDialOption(func(o *options) {
        dealAmqpTable(o.ExOpt.Arguments, exchangeOpt.Arguments)
        o.ExOpt = exchangeOpt
    })
}

// WithDeadExchangeOpt 死信交换机配置, 只有配置了死信队列才生效
func WithDeadExchangeOpt(exchangeOpt ExchangeDeclareOpt) DialOption {
    return newFuncDialOption(func(o *options) {
        dealAmqpTable(o.DeadExOpt.Arguments, exchangeOpt.Arguments)
        o.DeadExOpt = exchangeOpt
    })
}

// WithQueueOpt 队列配置
func WithQueueOpt(queueOpt QueueDeclareOpt) DialOption {
    return newFuncDialOption(func(o *options) {
        dealAmqpTable(o.QOpt.Arguments, queueOpt.Arguments)
        o.QOpt = queueOpt
    })
}

// WithDeadQueueOpt 死信队列配置, 只有配置了死信队列才生效
func WithDeadQueueOpt(queueOpt QueueDeclareOpt) DialOption {
    return newFuncDialOption(func(o *options) {
        dealAmqpTable(o.DeadQOpt.Arguments, queueOpt.Arguments)
        o.DeadQOpt = queueOpt
    })
}

// WithQueueBindOpt 队列绑定配置
func WithQueueBindOpt(queueBindOpt QueueBindOpt) DialOption {
    return newFuncDialOption(func(o *options) {
        dealAmqpTable(o.QBind.Arguments, queueBindOpt.Arguments)
        o.QBind = queueBindOpt
    })
}

// WithDeadQueueBindOpt 死信队列绑定配置, 只有配置了死信队列才生效
func WithDeadQueueBindOpt(queueBindOpt QueueBindOpt) DialOption {
    return newFuncDialOption(func(o *options) {
        dealAmqpTable(o.DeadQBind.Arguments, queueBindOpt.Arguments)
        o.DeadQBind = queueBindOpt
    })
}

// WithConsumeOpt Consume 参数
func WithConsumeOpt(consumeOpt ConsumeOpt) DialOption {
    return newFuncDialOption(func(o *options) {
        dealAmqpTable(o.ConsumeOpt.Arguments, consumeOpt.Arguments)
        o.ConsumeOpt = consumeOpt
    })
}

func dealAmqpTable(arguments, dstArguments amqp.Table) {
    if arguments != nil {
        if dstArguments == nil {
            dstArguments = make(amqp.Table, len(arguments))
        }
        for k, v := range arguments {
            if _, ok := dstArguments[k]; !ok {
                dstArguments[k] = v
            }
        }
        // 如果队列设置了死信队列, x-dead-letter-exchange 强制使用该交换机名
        if deadExchange, ok := arguments["x-dead-letter-exchange"]; ok {
            dstArguments["x-dead-letter-exchange"] = deadExchange
        }
    }
}
