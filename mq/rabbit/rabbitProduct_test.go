/*
Author ：zaniu(zzaniu@126.com)
Time   ：2024/3/15 15:15
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

package rabbit

import (
    "fmt"
    amqp "github.com/rabbitmq/amqp091-go"
    "testing"
)

var rbInfo = RbInfo{
    Addr:         "amqp://admin:123456@172.18.2.249:5673/test",
    ExchangeName: "test_exchange0315",
    QueueName:    "test_queue0315",
    RouteKey:     "test_route0315",
}

func TestNewAndInitRabbit(t *testing.T) {
    ch := make(chan string, 100)
    endCh := make(chan struct{})
    go func() {
        var index int
        client, err := NewAndInitRabbitClient(rbInfo, func(delivery amqp.Delivery) {
            s := string(delivery.Body)
            if fmt.Sprintf("%d, test", index) != s {
                t.Error("delivery body is wrong")
            }
            if err := delivery.Ack(false); err != nil {
                t.Error(err)
            }
            index++
            if index >= 100 {
                close(endCh)
            }
        })
        if err != nil {
            t.Error(err)
        }
        client.Consume(1)
    }()
    go func() {
        conn, err := NewAndInitRabbitServer(rbInfo)
        if err != nil {
            t.Error(err)
        }

        for s := range ch {
            conn.Publish([]byte(s))
        }
    }()
    for i := 0; i < 100; i++ {
        ch <- fmt.Sprintf("%d, test", i)
    }
    close(ch)
    <-endCh
}
