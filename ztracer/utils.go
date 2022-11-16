/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/9 22:16
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
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	gcodes "google.golang.org/grpc/codes"
	"net"
	"strings"
)

const (
	localhost = "127.0.0.1"
	name      = "zrpc"
	// GRPCStatusCodeKey is convention for numeric status code of a gRPC request.
	GRPCStatusCodeKey = attribute.Key("rpc.grpc.status_code")
	// RPCNameKey is the name of message transmitted or received.
	RPCNameKey = attribute.Key("name")
	// RPCMessageTypeKey is the type of message transmitted or received.
	RPCMessageTypeKey = attribute.Key("message.type")
	// RPCMessageIDKey is the identifier of message transmitted or received.
	RPCMessageIDKey = attribute.Key("message.id")
	// RPCMessageCompressedSizeKey is the compressed size of the message transmitted or received in bytes.
	RPCMessageCompressedSizeKey = attribute.Key("message.compressed_size")
	// RPCMessageUncompressedSizeKey is the uncompressed size of the message
	// transmitted or received in bytes.
	RPCMessageUncompressedSizeKey = attribute.Key("message.uncompressed_size")
)

var (
	// RPCSystemGRPC is the semantic convention for gRPC as the remoting system.
	RPCSystemGRPC = semconv.RPCSystemKey.String("grpc")
	// RPCNameMessage is the semantic convention for a message named message.
	RPCNameMessage = RPCNameKey.String("message")
	// RPCMessageTypeSent is the semantic conventions for sent RPC message types.
	RPCMessageTypeSent = RPCMessageTypeKey.String("SENT")
	// RPCMessageTypeReceived is the semantic conventions for the received RPC message types.
	RPCMessageTypeReceived = RPCMessageTypeKey.String("RECEIVED")
)

// ParseFullMethod 抄的 就是设置一些属性, 解析一些 Name 方法名 等
// returns the method name and attributes.
func ParseFullMethod(fullMethod string) (string, []attribute.KeyValue) {
	name := strings.TrimLeft(fullMethod, "/")
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		// Invalid format, does not follow `/package.service/method`.
		return name, []attribute.KeyValue(nil)
	}

	var attrs []attribute.KeyValue
	if service := parts[0]; service != "" {
		attrs = append(attrs, semconv.RPCServiceKey.String(service))
	}
	if method := parts[1]; method != "" {
		attrs = append(attrs, semconv.RPCMethodKey.String(method))
	}

	return name, attrs
}

// PeerAttr returns the peer attributes.
func PeerAttr(addr string) []attribute.KeyValue {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return []attribute.KeyValue(nil)
	}

	if len(host) == 0 {
		host = localhost
	}

	return []attribute.KeyValue{
		semconv.NetPeerIPKey.String(host),
		semconv.NetPeerPortKey.String(port),
	}
}

// StatusCodeAttr 这里写死了是一个 rpc.grpc.status_code
func StatusCodeAttr(c gcodes.Code) attribute.KeyValue {
	return GRPCStatusCodeKey.Int64(int64(c))
}
