/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/9 14:58
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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"io"
)

// assert that metadataSupplier implements the TextMapCarrier interface
var _ propagation.TextMapCarrier = new(metadataSupplier)

type (
	// Trace 一个大的系统就用一个就行了, 不然显示就会有好几个, 但是数据却是重复的
	Trace struct {
		Endpoint string `yaml:"Endpoint"`
		Name     string `yaml:"Name"`
		Model    string `yaml:"Model"`
	}

	Option struct {
		Name string `yaml:"Name"`
	}

	metadataSupplier struct {
		metadata *metadata.MD
	}
)

func (s *metadataSupplier) Get(key string) string {
	values := s.metadata.Get(key)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (s *metadataSupplier) Set(key, value string) {
	s.metadata.Set(key, value)
}

func (s *metadataSupplier) Keys() []string {
	out := make([]string, 0, len(*s.metadata))
	for key := range *s.metadata {
		out = append(out, key)
	}

	return out
}

// Inject injects the metadata into ctx.
func Inject(ctx context.Context, p propagation.TextMapPropagator, metadata *metadata.MD) {
	p.Inject(ctx, &metadataSupplier{
		metadata: metadata,
	})
}

// Extract extracts the metadata from ctx.
func Extract(ctx context.Context, p propagation.TextMapPropagator, metadata *metadata.MD) (
	baggage.Baggage, trace.SpanContext) {
	ctx = p.Extract(ctx, &metadataSupplier{
		metadata: metadata,
	})

	return baggage.FromContext(ctx), trace.SpanContextFromContext(ctx)
}

// SetJaegerTracerProvider 官方栗子里面抄的, 按照 kratos 的改动了一丢丢
// 设置一个全局的 Jaeger Tracer
func SetJaegerTracerProvider(tra Trace) error {
	// 创建 Jaeger 接收者
	var (
		exp  *jaeger.Exporter
		err  error
		opts []tracesdk.TracerProviderOption
	)
	opts = []tracesdk.TracerProviderOption{
		// 设置一些记录信息
		tracesdk.WithResource(resource.NewSchemaless(
			// 设置名字
			semconv.ServiceNameKey.String(tra.Name),
			// 设置环境名
			attribute.String("env", tra.Model),
		))}
	if len(tra.Endpoint) > 0 {
		exp, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(tra.Endpoint)))
		if err != nil {
			return err
		}
		// ParentBased: 设置为1, 表示所有的都上报, 设置为0则不上报, WithBatcher: 分批上报
		opts = append(opts, tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(1))), tracesdk.WithBatcher(exp))
	} else {
		opts = append(opts, tracesdk.WithSampler(tracesdk.NeverSample()), tracesdk.WithBatcher(tracetest.NewNoopExporter()))
	}
	tp := tracesdk.NewTracerProvider(opts...)
	// 设置全局 tracer
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return nil
}

func SetNoopTracerProvider() {
	tp := tracesdk.NewTracerProvider(tracesdk.WithSampler(tracesdk.NeverSample()), tracesdk.WithBatcher(tracetest.NewNoopExporter()))
	// 设置全局 tracer
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
}

// SetOutTracerProvider 上报到 io.Writer
// Deprecated
func SetOutTracerProvider(w io.Writer) error {
	// TODO 还可以搞一个输出到文件的, 但是感觉意义不太大
	//  import "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	// var (
	//     opts []tracesdk.TracerProviderOption
	// )
	// exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint(), stdouttrace.WithWriter(w))
	// if err != nil {
	//     return err
	// }
	// opts = []tracesdk.TracerProviderOption{
	//     // 设置一些记录信息
	//     tracesdk.WithResource(resource.NewSchemaless(
	//         // 设置名字
	//         semconv.ServiceNameKey.String(tra.Name),
	//         // 设置环境名
	//         attribute.String("env", tra.Model),
	//     ))}
	//
	// opts = append(opts, tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(1))), tracesdk.WithBatcher(exp))
	// tp := tracesdk.NewTracerProvider(opts...)
	// // 设置全局 tracer
	// otel.SetTracerProvider(tp)
	// otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return nil
}

func SpanInfo(fullMethod, peerAddress string) (string, []attribute.KeyValue) {
	// 设置 rpc.system 属性为 grpc (自定义的, 可不设置. 但是设置了的话, 可以一下子看出来是 http 还是 grpc)
	attrs := []attribute.KeyValue{RPCSystemGRPC}
	name, mAttrs := ParseFullMethod(fullMethod)
	attrs = append(attrs, mAttrs...)
	attrs = append(attrs, PeerAttr(peerAddress)...)
	return name, attrs
}

// PeerFromCtx returns the peer from ctx.
func PeerFromCtx(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}

	return p.Addr.String()
}

// GetTrace 获取 Tracer
// 第一次会生成, 后续会直接使用
func GetTrace() trace.Tracer {
	return otel.Tracer(name)
}
