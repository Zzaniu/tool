/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/9 22:58
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

package ztracer

import (
    "context"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/protobuf/proto"
)

const messageEvent = "message"

var (
    // MessageSent is the type of sent messages.
    MessageSent = messageType(RPCMessageTypeSent)
    // MessageReceived is the type of received messages.
    MessageReceived = messageType(RPCMessageTypeReceived)
)

type messageType attribute.KeyValue

// Event adds an event of the messageType to the span associated with the
// passed context with id and size (if message is a proto message).
func (m messageType) Event(ctx context.Context, id int, message interface{}) {
    span := trace.SpanFromContext(ctx)
    if p, ok := message.(proto.Message); ok {
        span.AddEvent(messageEvent, trace.WithAttributes(
            attribute.KeyValue(m),
            RPCMessageIDKey.Int(id),
            RPCMessageUncompressedSizeKey.Int(proto.Size(p)),
        ))
    } else {
        span.AddEvent(messageEvent, trace.WithAttributes(
            attribute.KeyValue(m),
            RPCMessageIDKey.Int(id),
		))
    }
}
