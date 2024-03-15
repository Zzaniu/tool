package rabbit

import (
    "fmt"
    "github.com/Zzaniu/tool/zlog"
    amqp "github.com/rabbitmq/amqp091-go"
    "golang.org/x/xerrors"
    "time"
)

type (
    RbMqClient struct {
        rbInfo              RbInfo
        connection          *amqp.Connection
        done                chan struct{}
        coonNotifyClose     chan *amqp.Error
        coonNotifyConnected chan struct{}
        callBack            func(amqp.Delivery)
    }
)

func (rabbitProduct *RbMqClient) InitRabbitProduct() {
    go rabbitProduct.handleReconnect()
}

// handleReconnect 处理重新连接
func (rabbitProduct *RbMqClient) handleReconnect() {
    for {
        // 如果连接断开了，每隔10S重新连接
        if rabbitProduct.connection == nil || rabbitProduct.connection.IsClosed() {
            for !rabbitProduct.connect(rabbitProduct.rbInfo.Addr) {
                fmt.Println("连接失败了")
                select {
                case <-rabbitProduct.done:
                    return
                default:
                    time.Sleep(rabbitProduct.rbInfo.opts.ReconnectDelay)
                }
            }
        }
        select {
        case <-rabbitProduct.done:
            return
        case <-rabbitProduct.coonNotifyClose: // 一般来说是网络断了
            zlog.Errorf("网络波动，或者是断网了。。。 %0fs后进行重连", rabbitProduct.rbInfo.opts.ReconnectDelay.Seconds())
        }
    }
}

// connect 连接MQ
func (rabbitProduct *RbMqClient) connect(addr string) bool {
    defer func() {
        if e := recover(); e != nil {
            if err, ok := e.(error); ok {
                zlog.Error("connect 发生错误, err = %+v", xerrors.Errorf("%w", err.(error)))
            } else {
                zlog.Error("connect 发生错误, err = %v", e)
            }
        }
    }()
    conn, err := amqp.Dial(addr)
    if err != nil {
        zlog.Errorf("RabbitMq连接失败, err = %+v\n", xerrors.Errorf("%w", err))
        return false
    }
    rabbitProduct.connection = conn
    ch, err := rabbitProduct.connection.Channel()
    if err != nil {
        zlog.Errorf("Channel连接失败, err = %+v\n", xerrors.Errorf("%w", err))
        return false
    }
    defer ch.Close()
    if err = ch.ExchangeDeclare(
        rabbitProduct.rbInfo.ExchangeName,
        rabbitProduct.rbInfo.opts.ExOpt.ExchangeType, // 默认路由模式
        rabbitProduct.rbInfo.opts.Durable,            // 持久化
        rabbitProduct.rbInfo.opts.ExOpt.AutoDelete,   // 使用完后删除队列
        rabbitProduct.rbInfo.opts.ExOpt.Internal,     // 这个是有用的哈, 比如说只希望别人接收到消息, 不希望他能发送消息
        rabbitProduct.rbInfo.opts.ExOpt.NoWait,       // 是否阻塞
        rabbitProduct.rbInfo.opts.ExOpt.Arguments); err != nil {
        zlog.Fatalf("交换机`%v`声明失败, err = %+v\n", rabbitProduct.rbInfo.ExchangeName, xerrors.Errorf("%w", err))
    }
    if len(rabbitProduct.rbInfo.DeadExchangeName) > 0 && len(rabbitProduct.rbInfo.DeadQueueName) > 0 {
        // 声明死信交换机
        if err = ch.ExchangeDeclare(
            rabbitProduct.rbInfo.DeadExchangeName,
            rabbitProduct.rbInfo.opts.DeadExOpt.ExchangeType, // 死信队列默认广播模式, 其实也可以用路由模式
            rabbitProduct.rbInfo.opts.DeadDurable,
            rabbitProduct.rbInfo.opts.DeadExOpt.AutoDelete,
            rabbitProduct.rbInfo.opts.DeadExOpt.Internal,
            rabbitProduct.rbInfo.opts.DeadExOpt.NoWait,
            rabbitProduct.rbInfo.opts.DeadExOpt.Arguments); err != nil {
            zlog.Fatalf("死信交换机`%v`声明失败, err = %+v\n", rabbitProduct.rbInfo.DeadExchangeName, xerrors.Errorf("%w", err))
        }
        // 声明死信队列
        if _, err = ch.QueueDeclare(
            rabbitProduct.rbInfo.DeadQueueName,
            rabbitProduct.rbInfo.opts.DeadDurable,
            rabbitProduct.rbInfo.opts.DeadQOpt.AutoDelete,
            rabbitProduct.rbInfo.opts.DeadQOpt.Exclusive,
            rabbitProduct.rbInfo.opts.DeadQOpt.NoWait,
            rabbitProduct.rbInfo.opts.DeadQOpt.Arguments); err != nil {
            zlog.Fatalf("死信队列`%v`声明失败, err = %+v\n", rabbitProduct.rbInfo.DeadQueueName, xerrors.Errorf("%w", err))
        }
        // 绑定死信队列与死信交换机
        if err = ch.QueueBind(
            rabbitProduct.rbInfo.DeadQueueName,
            rabbitProduct.rbInfo.DeadRouteKey,
            rabbitProduct.rbInfo.DeadExchangeName,
            rabbitProduct.rbInfo.opts.DeadQBind.NoWait,
            rabbitProduct.rbInfo.opts.DeadQBind.Arguments); err != nil {
            zlog.Fatalf("死信队列`%v-%v-%v`绑定失败, err = %+v\n", rabbitProduct.rbInfo.DeadQueueName, rabbitProduct.rbInfo.DeadRouteKey, rabbitProduct.rbInfo.DeadExchangeName, xerrors.Errorf("%w", err))
        }
    }
    // 声明队列
    if _, err = ch.QueueDeclare(
        rabbitProduct.rbInfo.QueueName,
        rabbitProduct.rbInfo.opts.Durable,
        rabbitProduct.rbInfo.opts.QOpt.AutoDelete,
        rabbitProduct.rbInfo.opts.QOpt.Exclusive,
        rabbitProduct.rbInfo.opts.QOpt.NoWait,
        rabbitProduct.rbInfo.opts.QOpt.Arguments, // 为队列绑定死信交换机
    ); err != nil {
        zlog.Fatalf("队列`%v`声明失败, err = %+v\n", rabbitProduct.rbInfo.QueueName, xerrors.Errorf("%w", err))
    }
    if err = ch.QueueBind(
        rabbitProduct.rbInfo.QueueName,
        rabbitProduct.rbInfo.RouteKey,
        rabbitProduct.rbInfo.ExchangeName,
        rabbitProduct.rbInfo.opts.QBind.NoWait,
        rabbitProduct.rbInfo.opts.QBind.Arguments); err != nil {
        zlog.Fatalf("队列`%v-%v-%v`绑定失败, err = %+v\n", rabbitProduct.rbInfo.QueueName, rabbitProduct.rbInfo.RouteKey, rabbitProduct.rbInfo.ExchangeName, xerrors.Errorf("%w", err))
    }
    // 每次连上了都要重新注册 NotifyClose 监听connection关闭通知
    rabbitProduct.registerConnNotifyClose()
    rabbitProduct.coonNotifyConnected <- struct{}{}
    zlog.Info("连接成功!!!")
    return true
}

// registerConnNotifyClose 注册 NotifyClose 监听connection关闭通知
func (rabbitProduct *RbMqClient) registerConnNotifyClose() {
    rabbitProduct.coonNotifyClose = make(chan *amqp.Error)
    rabbitProduct.connection.NotifyClose(rabbitProduct.coonNotifyClose)
}

// Publish 发布消息
func (rabbitProduct *RbMqClient) Publish(msg []byte) bool {
    if rabbitProduct.connection.IsClosed() {
        zlog.Errorf("%+v\n", xerrors.New("RabbitMQ未连接"))
        return false
    }
    ch, err := rabbitProduct.connection.Channel()
    if err != nil {
        zlog.Fatalf("channel连接失败, err = %+v\n", xerrors.Errorf("%w", err))
    }
    defer ch.Close()
    if err = ch.Confirm(false); err != nil {
        zlog.Fatalf("开启确认模式失败, err = %+v\n", xerrors.Errorf("%w", err))
    }
    // 注意，这里在 ch 关闭的时候，也会给 notifyConfirm 发送消息。如果不处理的话，就会一直卡在那里了，就不会释放 ch 了
    notifyConfirm := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
    defer func() {
        go func() {
            for range notifyConfirm {
            }
        }()
    }()
    // DeliveryMode: 2 消息也要是持久化的才行
    if err = ch.Publish(rabbitProduct.rbInfo.ExchangeName, rabbitProduct.rbInfo.RouteKey, false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, Body: msg}); err != nil {
        zlog.Fatalf("发布消息失败, err = %+v\n", xerrors.Errorf("%w", err))
    }
    ticker := time.NewTicker(rabbitProduct.rbInfo.opts.ResendDelay) // 发送超时时间, 3s
    defer ticker.Stop()
    select {
    case confirm := <-notifyConfirm:
        if confirm.Ack {
            return true
        }
    case <-ticker.C: // 如果发布超时也没有收到broker的ack，就返回发布失败
        return false
    }
    return false
}

// PublishMulti 返回发送是否全部成功和发送成功的数量
func (rabbitProduct *RbMqClient) PublishMulti(msgs [][]byte) (bool, int) {
    if rabbitProduct.connection == nil || rabbitProduct.connection.IsClosed() {
        zlog.Errorf("%+v\n", xerrors.New("RabbitMQ未连接"))
        return false, 0
    }
    ch, err := rabbitProduct.connection.Channel()
    if err != nil {
        zlog.Fatalf("channel连接失败, err = %+v\n", xerrors.Errorf("%w", err))
    }
    defer ch.Close()
    if err = ch.Confirm(false); err != nil {
        zlog.Fatalf("开启确认模式失败, err = %+v\n", xerrors.Errorf("%w", err))
    }
    notifyConfirm := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
    defer func() {
        go func() {
            for range notifyConfirm {
            }
        }()
    }()
    for cnt, msg := range msgs {
        if err := ch.Publish(rabbitProduct.rbInfo.ExchangeName, rabbitProduct.rbInfo.RouteKey, false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, Body: msg}); err != nil {
            zlog.Fatalf("发布消息失败, err = %+v\n", xerrors.Errorf("%w", err))
        }
        ticker := time.NewTicker(rabbitProduct.rbInfo.opts.ResendDelay) // 发送超时时间, 3s
        select {
        case confirm := <-notifyConfirm:
            ticker.Stop()
            if confirm.Ack {
                continue
            }
        case <-ticker.C:
            ticker.Stop()
            return false, cnt
        }
    }
    return true, len(msgs)
}

// Consume 消费消息
// prefetchCount 预取数量，设置为1的话，可以实现性能高的服务器消费的数量多
func (rabbitProduct *RbMqClient) Consume(prefetchCount int) {
    defer func() {
        e := recover()
        if e != nil {
            if err, ok := e.(error); ok {
                zlog.Fatalf("消费端报错了, err = %+v\n", xerrors.Errorf("%w", err))
            } else {
                zlog.Fatalf("消费端报错了, err = %v\n", e)
            }
        }
    }()
    defer func() {
        if !rabbitProduct.connection.IsClosed() {
            _ = rabbitProduct.connection.Close()
        }
    }()
    for {
        ch, err := rabbitProduct.connection.Channel()
        if err != nil {
            // ch连接失败，休眠10S后重连
            zlog.Errorf("Channel连接失败, err = %+v\n", xerrors.Errorf("%w", err))
            time.Sleep(rabbitProduct.rbInfo.opts.ReconnectDelay)
            continue
        }
        if err = ch.Qos(prefetchCount, 0, false); err != nil {
            zlog.Fatalf("开启预取模式失败, err = %+v\n", xerrors.Errorf("%w", err))
        }
        delvers, err := ch.Consume(rabbitProduct.rbInfo.QueueName, amqp.ExchangeDirect, false, false, false, false, nil)
        if err != nil {
            zlog.Fatalf("开启消费失败, err = %+v\n", xerrors.Errorf("%w", err))
        }
        // 没有消息的时候就阻塞在这里。当连接断开的时候(断网)，这里直接退出，然后去判断是否重新连接上了，连接上了会再次启动监听
        for delver := range delvers {
            rabbitProduct.callBack(delver)
        }
        _ = ch.Close()
        select {
        case <-rabbitProduct.done:
            return
        case <-rabbitProduct.coonNotifyConnected:
        }
    }
}

// Close 关闭, 如果不关闭的话, channel 不会释放
func (rabbitProduct *RbMqClient) Close() {
    close(rabbitProduct.done)
    close(rabbitProduct.coonNotifyConnected)
    if !rabbitProduct.connection.IsClosed() {
        _ = rabbitProduct.connection.Close()
    }
}

// NewAndInitRabbitClient 新建消费端(消费消息)
func NewAndInitRabbitClient(rbInfo RbInfo, callBack func(amqp.Delivery), opts ...DialOption) (*RbMqClient, error) {
    if callBack == nil {
        return nil, CallBackError
    }
    product, err := NewRabbitProduct(rbInfo, callBack, opts...)
    if err != nil {
        return nil, err
    }
    product.InitRabbitProduct()
    <-product.coonNotifyConnected
    return product, nil
}

// NewAndInitRabbitServer 新建服务端(发布消息)
func NewAndInitRabbitServer(rbInfo RbInfo, opts ...DialOption) (*RbMqClient, error) {
    product, err := NewRabbitProduct(rbInfo, nil, opts...)
    if err != nil {
        return nil, err
    }
    product.InitRabbitProduct()
    <-product.coonNotifyConnected
    return product, nil
}
