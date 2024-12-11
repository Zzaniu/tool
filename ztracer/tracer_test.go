/*
Author ：zaniu(zzaniu@126.com)
Time   ：2024/12/11 17:52
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
    "go.opentelemetry.io/otel"
    tracesdk "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/trace"
    gcodes "google.golang.org/grpc/codes"
    "log"
    "testing"
    "time"
)

func TestSetJaegerTracerProvider(t *testing.T) {
    // 要先搭建一个jaeger环境，这里使用docker搭建一个jaeger2.1.0环境
    // docker run -d --rm --name jaeger2 -p 16686:16686 -p 4317:4317 -p 4318:4318 -p 5778:5778 -p 9411:9411 jaegertracing/jaeger:2.1.0
    // 然后在浏览器中访问 http://ip:16686/ 即可看到jaeger的界面
    t.Run("jaeger", func(t *testing.T) {
        tra := Trace{
            Endpoint: "http://172.18.2.249:4318/v1/traces", // 4318 是 http 协议的端口, 4317 是 grpc 协议的端口
            Name:     "Better",
            Model:    "Dev",
        }
        if err := SetJaegerTracerProvider(tra); err != nil {
            panic(err)
        }
        tr := GetTrace()
        name, attr := SpanInfo("abcd", "test111")
        _, span := tr.Start(context.Background(), name, trace.WithSpanKind(trace.SpanKindClient),
            trace.WithAttributes(attr...))
        time.Sleep(100 * time.Millisecond)
        span.SetAttributes(StatusCodeAttr(gcodes.OK))
        span.End()
        tp := otel.GetTracerProvider()
        provider := tp.(*tracesdk.TracerProvider)
        if err := provider.Shutdown(context.Background()); err != nil {
            log.Fatalf("关闭 Tracer 提供器失败: %v", err)
        }
    })
}
